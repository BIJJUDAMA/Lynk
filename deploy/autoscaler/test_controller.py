"""
Unit tests for the Lynk Docker Compose Autoscaling Controller.
100% pure ASCII.
"""

import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(__file__))
from controller import choose_replicas, AutoscalerState


class TestChooseReplicas(unittest.TestCase):
    def test_overload_by_cpu(self):
        # CPU > 65% with sufficient host headroom should propose scale up
        target = choose_replicas(
            current=2,
            rps_per_replica=10.0,
            cpu_percent=72.0,
            memory_percent=50.0,
            host_memory_headroom=45.0,
            active_connections=100,
            minimum=2,
            maximum=6,
        )
        self.assertEqual(target, 3)

    def test_overload_by_rps(self):
        # RPS > 40 with sufficient host headroom should propose scale up
        target = choose_replicas(
            current=2,
            rps_per_replica=55.0,
            cpu_percent=40.0,
            memory_percent=50.0,
            host_memory_headroom=30.0,
            active_connections=100,
            minimum=2,
            maximum=6,
        )
        self.assertEqual(target, 3)

    def test_overload_by_connections(self):
        # Active connections = 4000 across 2 replicas (2000/replica > 1500)
        target = choose_replicas(
            current=2,
            rps_per_replica=10.0,
            cpu_percent=25.0,
            memory_percent=50.0,
            host_memory_headroom=45.0,
            active_connections=4000,
            minimum=2,
            maximum=6,
        )
        self.assertEqual(target, 3)

    def test_severe_overload_double_step(self):
        # Severe connection surge (3500/replica > 3000) steps up by 2
        target = choose_replicas(
            current=2,
            rps_per_replica=10.0,
            cpu_percent=25.0,
            memory_percent=50.0,
            host_memory_headroom=45.0,
            active_connections=7000,
            minimum=2,
            maximum=6,
        )
        self.assertEqual(target, 4)

    def test_max_replicas_ceiling(self):
        # Should never exceed maximum configured replicas
        target = choose_replicas(
            current=6,
            rps_per_replica=100.0,
            cpu_percent=95.0,
            memory_percent=80.0,
            host_memory_headroom=40.0,
            active_connections=20000,
            minimum=2,
            maximum=6,
        )
        self.assertEqual(target, 6)

    def test_host_memory_safety_floor(self):
        # CRITICAL SAFETY: If host memory headroom < 20%, NEVER scale up!
        target = choose_replicas(
            current=3,
            rps_per_replica=60.0,
            cpu_percent=85.0,
            memory_percent=85.0,
            host_memory_headroom=12.0,  # Below 20% safety floor!
            active_connections=5000,
            minimum=2,
            maximum=6,
        )
        self.assertEqual(target, 3, "Must not scale up when host headroom is critically low")

    def test_underutilized_scale_down(self):
        # Underutilized across all dimensions should propose scale down
        target = choose_replicas(
            current=4,
            rps_per_replica=5.0,
            cpu_percent=15.0,
            memory_percent=40.0,
            host_memory_headroom=60.0,
            active_connections=200,
            minimum=2,
            maximum=6,
        )
        self.assertEqual(target, 3)

    def test_min_replicas_floor(self):
        # Should never scale down below minimum configured replicas
        target = choose_replicas(
            current=2,
            rps_per_replica=0.0,
            cpu_percent=2.0,
            memory_percent=20.0,
            host_memory_headroom=80.0,
            active_connections=0,
            minimum=2,
            maximum=6,
        )
        self.assertEqual(target, 2)

    def test_balanced_steady_state(self):
        # Moderate load should maintain current replica count
        target = choose_replicas(
            current=3,
            rps_per_replica=25.0,
            cpu_percent=50.0,
            memory_percent=65.0,
            host_memory_headroom=50.0,
            active_connections=1200,
            minimum=2,
            maximum=6,
        )
        self.assertEqual(target, 3)


class TestAutoscalerState(unittest.TestCase):
    def test_scale_up_sustained_window(self):
        state = AutoscalerState(
            scale_up_sustained_sec=20,
            scale_down_sustained_sec=300,
            scale_up_cooldown_sec=30,
            scale_down_cooldown_sec=180,
        )

        now = 1000.0
        # First high reading without is_urgent: should wait for sustained timer
        res = state.evaluate_decision(current_replicas=2, proposed_replicas=3, now=now, is_urgent=False)
        self.assertEqual(res, 2, "Should wait for sustained window")

        # 10s later: still in sustained window
        res = state.evaluate_decision(current_replicas=2, proposed_replicas=3, now=now + 10.0, is_urgent=False)
        self.assertEqual(res, 2, "Should still wait for sustained window")

        # 25s later: sustained window elapsed, should trigger scale up!
        res = state.evaluate_decision(current_replicas=2, proposed_replicas=3, now=now + 25.0, is_urgent=False)
        self.assertEqual(res, 3, "Should trigger scale up after sustained window")

    def test_urgent_scale_up_bypasses_sustained_window(self):
        state = AutoscalerState(
            scale_up_sustained_sec=20,
            scale_down_sustained_sec=300,
            scale_up_cooldown_sec=30,
            scale_down_cooldown_sec=180,
        )

        now = 1000.0
        # With is_urgent=True, scale up happens immediately on the very first sample
        res = state.evaluate_decision(current_replicas=2, proposed_replicas=4, now=now, is_urgent=True)
        self.assertEqual(res, 4, "Urgent scale up must bypass sustained window immediately")

    def test_scale_down_cooldown(self):
        state = AutoscalerState(
            scale_up_sustained_sec=20,
            scale_down_sustained_sec=300,
            scale_up_cooldown_sec=30,
            scale_down_cooldown_sec=180,
        )

        now = 2000.0
        # Trigger a scale up
        state.last_scale_time = now
        state.last_scale_action = "up"

        # Proposed scale down during cooldown should be ignored
        res = state.evaluate_decision(current_replicas=4, proposed_replicas=3, now=now + 60.0)
        self.assertEqual(res, 4, "Must obey scale down cooldown window")


if __name__ == "__main__":
    unittest.main()
