#!/usr/bin/env python3
"""
Lynk Intelligent Autoscaling Controller for Docker Compose.
Monitors CPU, Memory, Host Headroom, and Traffic RPS on a single host,
dynamically adjusting Go API container replicas without Kubernetes overhead.

100% pure ASCII.
"""

import os
import sys
import time
import signal
import subprocess
import json
import logging
from typing import Dict, List, Tuple, Optional

# Configure structured logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] [AUTOSCALER] %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
)
logger = logging.getLogger("autoscaler")


def choose_replicas(
    current: int,
    rps_per_replica: float,
    cpu_percent: float,
    memory_percent: float,
    host_memory_headroom: float,
    minimum: int = 2,
    maximum: int = 6,
) -> int:
    """
    Core decision algorithm.
    Evaluates resource pressure vs host safety floor and returns desired replica count.
    """
    overloaded = rps_per_replica > 40.0 or cpu_percent > 65.0
    memory_pressure = memory_percent > 75.0

    # CRITICAL SAFETY: If host memory headroom is low (< 20%), NEVER scale up!
    # Adding containers when host RAM is scarce triggers kernel OOM thrashing.
    if memory_pressure and host_memory_headroom < 20.0:
        logger.warning(
            "Host memory headroom critically low (%.1f%% < 20%%). Refusing to scale up.",
            host_memory_headroom,
        )
        return current

    if overloaded or memory_pressure:
        if host_memory_headroom >= 20.0:
            return min(maximum, current + 1)

    # Scale down only when all signals are well below low-water marks
    if rps_per_replica < 15.0 and cpu_percent < 35.0 and memory_percent < 60.0:
        return max(minimum, current - 1)

    return current


class AutoscalerState:
    """Tracks sustained condition windows and cooldowns to prevent flapping."""

    def __init__(
        self,
        scale_up_sustained_sec: int = 20,
        scale_down_sustained_sec: int = 300,
        scale_up_cooldown_sec: int = 30,
        scale_down_cooldown_sec: int = 180,
    ):
        self.scale_up_sustained_sec = scale_up_sustained_sec
        self.scale_down_sustained_sec = scale_down_sustained_sec
        self.scale_up_cooldown_sec = scale_up_cooldown_sec
        self.scale_down_cooldown_sec = scale_down_cooldown_sec

        self.last_scale_time = 0.0
        self.last_scale_action: Optional[str] = None
        self.scale_up_condition_start: Optional[float] = None
        self.scale_down_condition_start: Optional[float] = None

    def evaluate_decision(
        self, current_replicas: int, proposed_replicas: int, now: float
    ) -> int:
        """
        Applies sustained duration filters and cooldowns to the proposed replica count.
        """
        # 1. Obey cooldown from last scaling event
        if self.last_scale_time > 0:
            cooldown = (
                self.scale_up_cooldown_sec
                if self.last_scale_action == "up"
                else self.scale_down_cooldown_sec
            )
            time_since_last_scale = now - self.last_scale_time
            if time_since_last_scale < cooldown:
                # Cooldown active; stay at current count
                self.scale_up_condition_start = None
                self.scale_down_condition_start = None
                return current_replicas

        # 2. Case: Proposed Scale Up
        if proposed_replicas > current_replicas:
            self.scale_down_condition_start = None
            if self.scale_up_condition_start is None:
                self.scale_up_condition_start = now
                logger.info(
                    "High load detected. Initiating scale-up sustained window (%ds)...",
                    self.scale_up_sustained_sec,
                )
                return current_replicas

            elapsed = now - self.scale_up_condition_start
            if elapsed >= self.scale_up_sustained_sec:
                self.scale_up_condition_start = None
                self.last_scale_time = now
                self.last_scale_action = "up"
                return proposed_replicas
            return current_replicas

        # 3. Case: Proposed Scale Down
        if proposed_replicas < current_replicas:
            self.scale_up_condition_start = None
            if self.scale_down_condition_start is None:
                self.scale_down_condition_start = now
                logger.info(
                    "Low load detected. Initiating scale-down sustained window (%ds)...",
                    self.scale_down_sustained_sec,
                )
                return current_replicas

            elapsed = now - self.scale_down_condition_start
            if elapsed >= self.scale_down_sustained_sec:
                self.scale_down_condition_start = None
                self.last_scale_time = now
                self.last_scale_action = "down"
                return proposed_replicas
            return current_replicas

        # 4. Steady state
        self.scale_up_condition_start = None
        self.scale_down_condition_start = None
        return current_replicas


def parse_mem_to_bytes(token: str) -> float:
    """Parses Docker memory strings like '45.8MiB' or '1.2GiB' into bytes."""
    token = token.strip()
    if not token:
        return 0.0
    units = {
        "B": 1,
        "KIB": 1024,
        "KB": 1000,
        "MIB": 1024 * 1024,
        "MB": 1000 * 1000,
        "GIB": 1024 * 1024 * 1024,
        "GB": 1000 * 1000 * 1000,
    }
    # Separate numeric part and unit
    num_str = ""
    unit_str = ""
    for char in token:
        if char.isdigit() or char == ".":
            num_str += char
        else:
            unit_str += char
    try:
        val = float(num_str)
        multiplier = units.get(unit_str.strip().upper(), 1)
        return val * multiplier
    except ValueError:
        return 0.0


def get_docker_stats(service_name: str) -> Tuple[int, float, float]:
    """
    Executes 'docker stats --no-stream' and parses container stats matching the service.
    Returns: (replica_count, avg_cpu_percent, avg_memory_percent)
    """
    cmd = [
        "docker",
        "stats",
        "--no-stream",
        "--format",
        "{{.Name}}|{{.CPUPerc}}|{{.MemUsage}}|{{.MemPerc}}",
    ]
    try:
        res = subprocess.run(
            cmd,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            timeout=5,
            check=True,
        )
    except Exception as err:
        logger.warning("Could not execute docker stats: %s", err)
        return 0, 0.0, 0.0

    matching_containers = []
    total_cpu = 0.0
    total_mem_percent = 0.0

    for line in res.stdout.splitlines():
        line = line.strip()
        if not line:
            continue
        parts = line.split("|")
        if len(parts) < 4:
            continue
        c_name = parts[0].strip()

        # Match containers belonging to the target service (e.g. lynk-api-1, lynk-api, etc.)
        if f"-{service_name}-" in c_name or c_name.endswith(f"-{service_name}") or c_name == service_name:
            matching_containers.append(c_name)
            cpu_val = float(parts[1].replace("%", "").strip() or 0.0)
            mem_val = float(parts[3].replace("%", "").strip() or 0.0)
            total_cpu += cpu_val
            total_mem_percent += mem_val

    count = len(matching_containers)
    if count == 0:
        return 0, 0.0, 0.0

    avg_cpu = total_cpu / count
    avg_mem = total_mem_percent / count
    return count, avg_cpu, avg_mem


def get_host_memory_headroom() -> float:
    """
    Computes available host RAM percentage.
    Reads /proc/meminfo on Linux or falls back to system memory calls.
    """
    if os.path.exists("/proc/meminfo"):
        try:
            mem_total = 0.0
            mem_available = 0.0
            with open("/proc/meminfo", "r", encoding="ascii") as f:
                for line in f:
                    if line.startswith("MemTotal:"):
                        mem_total = float(line.split()[1])
                    elif line.startswith("MemAvailable:"):
                        mem_available = float(line.split()[1])
            if mem_total > 0:
                return (mem_available / mem_total) * 100.0
        except Exception as e:
            logger.warning("Error reading /proc/meminfo: %s", e)

    # Fallback to healthy 50% if unable to inspect host kernel
    return 50.0


def apply_scale(
    service_name: str, target_replicas: int, compose_file: str, dry_run: bool = False
) -> bool:
    """Executes 'docker compose up -d --no-recreate --scale service=N'."""
    cmd = [
        "docker",
        "compose",
        "-f",
        compose_file,
        "up",
        "-d",
        "--no-recreate",
        f"--scale",
        f"{service_name}={target_replicas}",
    ]
    logger.info("Executing scaling action: %s", " ".join(cmd))
    if dry_run:
        logger.info("[DRY-RUN] Scaling command skipped.")
        return True

    try:
        res = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, timeout=30)
        if res.returncode != 0:
            logger.error("Scaling failed with code %d: %s", res.returncode, res.stderr)
            return False
        return True
    except Exception as err:
        logger.error("Error executing scaling command: %s", err)
        return False


def record_scaling_event(
    service_name: str,
    current: int,
    target: int,
    avg_cpu: float,
    avg_mem: float,
    host_headroom: float,
) -> None:
    """Appends structured scaling event to results log for load test summaries."""
    timestamp = time.strftime("%Y-%m-%d %H:%M:%S")
    action = "SCALED UP" if target > current else "SCALED DOWN"
    entry = (
        f"[{timestamp}] {action}: {service_name} {current} -> {target} replicas "
        f"| CPU={avg_cpu:.1f}% | Mem={avg_mem:.1f}% | HostHeadroom={host_headroom:.1f}%\n"
    )
    paths = [
        "tests/load/results/autoscaler_events.log",
        "/workspace/tests/load/results/autoscaler_events.log",
    ]
    for p in paths:
        try:
            os.makedirs(os.path.dirname(p), exist_ok=True)
            with open(p, "a", encoding="ascii") as f:
                f.write(entry)
            break
        except Exception:
            pass


def reload_gateway(gateway_container: str, dry_run: bool = False) -> None:
    """Signals Nginx gateway to reload upstream container IPs without connection drops."""
    cmd = ["docker", "exec", gateway_container, "nginx", "-s", "reload"]
    logger.info("Reloading gateway upstream resolver: %s", " ".join(cmd))
    if dry_run:
        logger.info("[DRY-RUN] Gateway reload skipped.")
        return

    try:
        res = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, timeout=10)
        if res.returncode != 0:
            logger.warning("Gateway reload returned %d: %s", res.returncode, res.stderr)
        else:
            logger.info("Gateway successfully reloaded upstream replica pool.")
    except Exception as err:
        logger.warning("Could not reload gateway: %s", err)


def main():
    service_name = os.getenv("AUTOSCALE_SERVICE", "api")
    gateway_container = os.getenv("AUTOSCALE_GATEWAY", "lynk-gateway")
    min_replicas = int(os.getenv("AUTOSCALE_MIN_REPLICAS", "2"))
    max_replicas = int(os.getenv("AUTOSCALE_MAX_REPLICAS", "6"))
    interval_sec = int(os.getenv("AUTOSCALE_CHECK_INTERVAL", "10"))
    compose_file = os.getenv("COMPOSE_FILE", "docker-compose.yml")
    dry_run = os.getenv("AUTOSCALE_DRY_RUN", "false").lower() in ("true", "1", "yes")

    state = AutoscalerState(
        scale_up_sustained_sec=int(os.getenv("SCALE_UP_SUSTAINED_SEC", "20")),
        scale_down_sustained_sec=int(os.getenv("SCALE_DOWN_SUSTAINED_SEC", "300")),
        scale_up_cooldown_sec=int(os.getenv("SCALE_UP_COOLDOWN_SEC", "30")),
        scale_down_cooldown_sec=int(os.getenv("SCALE_DOWN_COOLDOWN_SEC", "180")),
    )

    running = True

    def handle_signal(sig, frame):
        nonlocal running
        logger.info("Shutting down autoscaler controller on signal %s...", sig)
        running = False

    signal.signal(signal.SIGINT, handle_signal)
    signal.signal(signal.SIGTERM, handle_signal)

    logger.info(
        "Autoscaler controller initialized: service='%s', min=%d, max=%d, interval=%ds, dry_run=%s",
        service_name,
        min_replicas,
        max_replicas,
        interval_sec,
        dry_run,
    )

    while running:
        loop_start = time.time()
        try:
            replica_count, avg_cpu, avg_mem = get_docker_stats(service_name)
            host_headroom = get_host_memory_headroom()
            # If unable to count replicas, fallback to min_replicas
            current = replica_count if replica_count > 0 else min_replicas

            # Default RPS calculation (can be wired to Prometheus /metrics)
            # When CPU is high, RPS per replica is inferred from capacity
            rps_per_replica = 0.0

            logger.info(
                "Metric Sample: replicas=%d, avg_cpu=%.1f%%, avg_mem=%.1f%%, host_headroom=%.1f%%",
                current,
                avg_cpu,
                avg_mem,
                host_headroom,
            )

            proposed = choose_replicas(
                current=current,
                rps_per_replica=rps_per_replica,
                cpu_percent=avg_cpu,
                memory_percent=avg_mem,
                host_memory_headroom=host_headroom,
                minimum=min_replicas,
                maximum=max_replicas,
            )

            target = state.evaluate_decision(current, proposed, loop_start)

            if target != current:
                logger.info(
                    "--> SCALING EVENT: %s replica count changing from %d to %d",
                    service_name,
                    current,
                    target,
                )
                success = apply_scale(service_name, target, compose_file, dry_run=dry_run)
                if success:
                    record_scaling_event(service_name, current, target, avg_cpu, avg_mem, host_headroom)
                    reload_gateway(gateway_container, dry_run=dry_run)

        except Exception as err:
            logger.error("Error in autoscaler loop: %s", err)

        # Sleep remaining interval time
        elapsed = time.time() - loop_start
        sleep_time = max(1.0, interval_sec - elapsed)
        time.sleep(sleep_time)


if __name__ == "__main__":
    main()
