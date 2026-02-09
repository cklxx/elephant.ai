"""Tests for timer_cli."""

from __future__ import annotations

import json
import sys
import time
from pathlib import Path
from unittest.mock import patch

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parent.parent.parent.parent))

from cli.timer.timer_cli import (
    _parse_delay,
    cancel_timer,
    list_timers,
    set_timer,
)


class TestParseDelay:
    def test_seconds(self):
        assert _parse_delay("30s") == 30

    def test_minutes(self):
        assert _parse_delay("5m") == 300

    def test_hours(self):
        assert _parse_delay("2h") == 7200

    def test_raw_number(self):
        assert _parse_delay("60") == 60

    def test_whitespace(self):
        assert _parse_delay("  10m  ") == 600


class TestSetTimer:
    def test_creates_timer(self, tmp_path):
        timer_file = tmp_path / "timers.json"
        with patch("cli.timer.timer_cli._TIMER_FILE", timer_file):
            result = set_timer({"delay": "5m", "task": "drink water"})
            assert result["success"] is True
            assert result["timer"]["task"] == "drink water"
            assert result["timer"]["status"] == "active"
            assert result["timer"]["id"].startswith("timer-")

            # Verify file written
            timers = json.loads(timer_file.read_text())
            assert len(timers) == 1

    def test_missing_delay(self, tmp_path):
        timer_file = tmp_path / "timers.json"
        with patch("cli.timer.timer_cli._TIMER_FILE", timer_file):
            result = set_timer({"task": "no delay"})
            assert result["success"] is False

    def test_missing_task(self, tmp_path):
        timer_file = tmp_path / "timers.json"
        with patch("cli.timer.timer_cli._TIMER_FILE", timer_file):
            result = set_timer({"delay": "5m"})
            assert result["success"] is False

    def test_multiple_timers(self, tmp_path):
        timer_file = tmp_path / "timers.json"
        with patch("cli.timer.timer_cli._TIMER_FILE", timer_file):
            set_timer({"delay": "5m", "task": "task 1"})
            set_timer({"delay": "10m", "task": "task 2"})
            timers = json.loads(timer_file.read_text())
            assert len(timers) == 2


class TestListTimers:
    def test_empty_list(self, tmp_path):
        timer_file = tmp_path / "timers.json"
        with patch("cli.timer.timer_cli._TIMER_FILE", timer_file):
            result = list_timers()
            assert result["success"] is True
            assert result["count"] == 0

    def test_marks_fired_timers(self, tmp_path):
        timer_file = tmp_path / "timers.json"
        past_timer = {
            "id": "timer-1",
            "task": "past",
            "fire_at": time.time() - 100,
            "status": "active",
        }
        timer_file.write_text(json.dumps([past_timer]))
        with patch("cli.timer.timer_cli._TIMER_FILE", timer_file):
            result = list_timers()
            assert result["timers"][0]["status"] == "fired"


class TestCancelTimer:
    def test_cancels_existing(self, tmp_path):
        timer_file = tmp_path / "timers.json"
        timer_file.write_text(json.dumps([{
            "id": "timer-42",
            "task": "test",
            "fire_at": time.time() + 1000,
            "status": "active",
        }]))
        with patch("cli.timer.timer_cli._TIMER_FILE", timer_file):
            result = cancel_timer({"id": "timer-42"})
            assert result["success"] is True
            timers = json.loads(timer_file.read_text())
            assert timers[0]["status"] == "cancelled"

    def test_cancel_nonexistent(self, tmp_path):
        timer_file = tmp_path / "timers.json"
        timer_file.write_text("[]")
        with patch("cli.timer.timer_cli._TIMER_FILE", timer_file):
            result = cancel_timer({"id": "timer-999"})
            assert result["success"] is False

    def test_missing_id(self, tmp_path):
        timer_file = tmp_path / "timers.json"
        with patch("cli.timer.timer_cli._TIMER_FILE", timer_file):
            result = cancel_timer({})
            assert result["success"] is False
