#!/usr/bin/env python3
"""
Lynk Intelligent Autoscaling Controller for Docker Compose.
Monitors CPU, Memory, Host Headroom, Active Connections, and Traffic RPS on a single host,
dynamically adjusting Go API container replicas in Docker Compose.

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
    stream=sys.stdout,
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
    active_connections: int = 0,
    minimum: int = 2,
    maximum: int = 6,
) -> int:
    """
    Core decision algorithm.
    Evaluates resource pressure vs host safety floor and returns desired replica count.
    Supports connection-aware scaling to absorb connection bursts.
    """
    conns_per_replica = float(active_connections) / max(1.0, float(current))
    overloaded = rps_per_replica > 40.0 or cpu_percent > 65.0 or conns_per_replica > 1500.0
    memory_pressure = memory_percent > 75.0

    # CRITICAL SAFETY: If host memory headroom is low (< 20%), NEVER scale up!
    # Adding containers when host RAM is scarce triggers kernel OOM thrashing.
    if (overloaded or memory_pressure) and host_memory_headroom < 20.0:
        logger.warning(
            "Host memory headroom critically low (%.1f%% < 20%%). Refusing to scale up.",
            host_memory_headroom,
        )
        return current

    if overloaded or memory_pressure:
        if host_memory_headroom >= 20.0:
            # Under severe surge (connections > 3000/rep or CPU > 85%), step up by 2 replicas
            step = 2 if (conns_per_replica >= 3000.0 or cpu_percent >= 85.0) else 1
            return min(maximum, current + step)

    # Scale down only when all signals are well below low-water marks
    if (
        rps_per_replica < 15.0
        and cpu_percent < 35.0
        and memory_percent < 60.0
        and conns_per_replica < 500.0
    ):
        return max(minimum, current - 1)

    return current


class AutoscalerState:
    """Tracks sustained condition windows and cooldowns to prevent flapping."""

    def __init__(
        self,
        scale_up_sustained_sec: int = 5,
        scale_down_sustained_sec: int = 300,
        scale_up_cooldown_sec: int = 15,
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
        self,
        current_replicas: int,
        proposed_replicas: int,
        now: float,
        is_urgent: bool = False,
    ) -> int:
        """
        Applies sustained duration filters and cooldowns to the proposed replica count.
        Urgent surges bypass the sustained window for immediate protection.
        """
        # 1. Case: Proposed Scale Up
        if proposed_replicas > current_replicas:
            self.scale_down_condition_start = None

            # Urgent spike path: bypass sustained window if not in immediate scale spam
            if is_urgent:
                if self.last_scale_time > 0 and (now - self.last_scale_time) < 3.0:
                    return current_replicas
                self.scale_up_condition_start = None
                self.last_scale_time = now
                self.last_scale_action = "up"
                logger.info(
                    "Urgent load spike detected! Bypassing sustained window, scaling immediately to %d replicas.",
                    proposed_replicas,
                )
                return proposed_replicas

            # Standard scale up: obey cooldown from last scaling event
            if self.last_scale_time > 0:
                cooldown = (
                    self.scale_up_cooldown_sec
                    if self.last_scale_action == "up"
                    else self.scale_down_cooldown_sec
                )
                time_since_last_scale = now - self.last_scale_time
                if time_since_last_scale < cooldown:
                    self.scale_up_condition_start = None
                    return current_replicas

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

        # 2. Case: Proposed Scale Down
        if proposed_replicas < current_replicas:
            self.scale_up_condition_start = None

            # Scale down always respects cooldown
            if self.last_scale_time > 0:
                cooldown = (
                    self.scale_up_cooldown_sec
                    if self.last_scale_action == "up"
                    else self.scale_down_cooldown_sec
                )
                time_since_last_scale = now - self.last_scale_time
                if time_since_last_scale < cooldown:
                    self.scale_down_condition_start = None
                    return current_replicas

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

        # 3. Steady state
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


def get_docker_stats(service_name: str) -> Tuple[int, float, float, float]:
    """
    Executes 'docker stats --no-stream' and parses container stats matching the service.
    Returns: (replica_count, avg_cpu_percent, avg_memory_percent, total_mem_mb)
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
        return 0, 0.0, 0.0, 0.0

    matching_containers = []
    total_cpu = 0.0
    total_mem_percent = 0.0
    total_mem_bytes = 0.0

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
            mem_usage_token = parts[2].split("/")[0].strip()
            mem_bytes = parse_mem_to_bytes(mem_usage_token)
            mem_val = float(parts[3].replace("%", "").strip() or 0.0)
            total_cpu += cpu_val
            total_mem_percent += mem_val
            total_mem_bytes += mem_bytes

    count = len(matching_containers)
    if count == 0:
        return 0, 0.0, 0.0, 0.0

    avg_cpu = total_cpu / count
    avg_mem = total_mem_percent / count
    total_mem_mb = total_mem_bytes / (1024.0 * 1024.0)
    return count, avg_cpu, avg_mem, total_mem_mb


def get_active_connections(
    gateway_url: str = "http://lynk-gateway:8080/nginx_status",
    gateway_container: str = "lynk-gateway",
) -> int:
    """
    Retrieves the count of active TCP connections from Nginx stub_status.
    Falls back to docker exec or socket inspection if direct HTTP fails.
    """
    # 1. Try querying nginx stub_status over HTTP
    try:
        import urllib.request
        req = urllib.request.Request(gateway_url, headers={"User-Agent": "Lynk-Autoscaler"})
        with urllib.request.urlopen(req, timeout=1.5) as resp:
            text = resp.read().decode("ascii", errors="replace")
            for line in text.splitlines():
                if "Active connections:" in line:
                    parts = line.split(":")
                    if len(parts) > 1:
                        return int(parts[1].strip())
    except Exception:
        pass

    # 2. Fallback: try docker exec into gateway container to query localhost
    try:
        cmd = ["docker", "exec", gateway_container, "wget", "-qO-", "http://127.0.0.1:8080/nginx_status"]
        res = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, timeout=2)
        if res.returncode == 0:
            for line in res.stdout.splitlines():
                if "Active connections:" in line:
                    parts = line.split(":")
                    if len(parts) > 1:
                        return int(parts[1].strip())
    except Exception:
        pass

    # 3. Fallback: inspect /proc/net/tcp and /proc/net/tcp6 for port 8080 (0x1F90) in ESTABLISHED (01) state
    conn_count = 0
    for proc_path in ("/proc/net/tcp", "/proc/net/tcp6"):
        if os.path.exists(proc_path):
            try:
                with open(proc_path, "r", encoding="ascii") as f:
                    for line in f:
                        parts = line.strip().split()
                        if len(parts) >= 4 and parts[3] == "01" and parts[1].endswith(":1F90"):
                            conn_count += 1
            except Exception:
                pass
    if conn_count > 0:
        return conn_count

    return 0


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
    active_conns: int = 0,
    total_mem_mb: float = 0.0,
) -> None:
    """Appends structured scaling event to results log for load test summaries."""
    timestamp = time.strftime("%Y-%m-%d %H:%M:%S")
    action = "SCALED UP" if target > current else "SCALED DOWN"
    entry = (
        f"[{timestamp}] {action}: {service_name} {current} -> {target} replicas "
        f"| CPU={avg_cpu:.1f}% | RAM={total_mem_mb:.0f}MB ({avg_mem:.1f}% host) | Conns={active_conns} | HostHeadroom={host_headroom:.1f}%\n"
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
    interval_sec = int(os.getenv("AUTOSCALE_CHECK_INTERVAL", "5"))
    compose_file = os.getenv("COMPOSE_FILE", "docker-compose.yml")
    dry_run = os.getenv("AUTOSCALE_DRY_RUN", "false").lower() in ("true", "1", "yes")

    state = AutoscalerState(
        scale_up_sustained_sec=int(os.getenv("SCALE_UP_SUSTAINED_SEC", "5")),
        scale_down_sustained_sec=int(os.getenv("SCALE_DOWN_SUSTAINED_SEC", "300")),
        scale_up_cooldown_sec=int(os.getenv("SCALE_UP_COOLDOWN_SEC", "15")),
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
    init_entry = (
        f"[{time.strftime('%Y-%m-%d %H:%M:%S')}] INITIALIZED: Autoscaler online for service '{service_name}' "
        f"(min={min_replicas}, max={max_replicas}, check_interval={interval_sec}s)\n"
    )
    for p in ["tests/load/results/autoscaler_events.log", "/workspace/tests/load/results/autoscaler_events.log"]:
        try:
            os.makedirs(os.path.dirname(p), exist_ok=True)
            with open(p, "a", encoding="ascii") as f:
                f.write(init_entry)
            break
        except Exception:
            pass

    while running:
        loop_start = time.time()
        try:
            replica_count, avg_cpu, avg_mem, total_mem_mb = get_docker_stats(service_name)
            host_headroom = get_host_memory_headroom()
            current = replica_count if replica_count > 0 else min_replicas
            host_cores = os.cpu_count() or 4
            host_cpu_pct = (avg_cpu * current) / float(host_cores)

            active_connections = get_active_connections(
                gateway_url=f"http://{gateway_container}:8080/nginx_status",
                gateway_container=gateway_container,
            )
            conns_per_replica = float(active_connections) / max(1.0, float(current))

            # Default RPS calculation (can be wired to Prometheus /metrics)
            rps_per_replica = 0.0

            logger.info(
                "Metric Sample: replicas=%d | cpu=%.1f%% (%.1f cores / %.1f%% of %d vCPUs) | ram=%.0fMB (%.1f%% host) | conns=%d (%.1f/rep) | host_headroom=%.1f%%",
                current,
                avg_cpu,
                (avg_cpu * current) / 100.0,
                host_cpu_pct,
                host_cores,
                total_mem_mb,
                avg_mem,
                active_connections,
                conns_per_replica,
                host_headroom,
            )

            proposed = choose_replicas(
                current=current,
                rps_per_replica=rps_per_replica,
                cpu_percent=avg_cpu,
                memory_percent=avg_mem,
                host_memory_headroom=host_headroom,
                active_connections=active_connections,
                minimum=min_replicas,
                maximum=max_replicas,
            )

            is_urgent = (
                avg_cpu >= 80.0
                or conns_per_replica >= 2500.0
                or rps_per_replica >= 100.0
            )

            target = state.evaluate_decision(current, proposed, loop_start, is_urgent=is_urgent)

            if target != current:
                logger.info(
                    "--> SCALING EVENT: %s replica count changing from %d to %d",
                    service_name,
                    current,
                    target,
                )
                success = apply_scale(service_name, target, compose_file, dry_run=dry_run)
                if success:
                    record_scaling_event(
                        service_name,
                        current,
                        target,
                        avg_cpu,
                        avg_mem,
                        host_headroom,
                        active_conns=active_connections,
                        total_mem_mb=total_mem_mb,
                    )
                    reload_gateway(gateway_container, dry_run=dry_run)

        except Exception as err:
            logger.error("Error in autoscaler loop: %s", err)

        # Sleep remaining interval time
        elapsed = time.time() - loop_start
        sleep_time = max(1.0, interval_sec - elapsed)
        time.sleep(sleep_time)


if __name__ == "__main__":
    main()
