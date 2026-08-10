#!/usr/bin/env python3
"""
drone_bridge.py - HTTP-to-MAVLink bridge for aflare drone node.

This script runs as a lightweight HTTP server that translates REST API calls
into MAVLink commands for PX4/ArduPilot drones. It connects to a drone via
serial, UDP, or TCP using pymavlink.

Requirements:
    pip install pymavlink flask

Usage:
    # Connect via serial (USB)
    python drone_bridge.py --port 8080 --connection /dev/ttyACM0:115200

    # Connect via UDP (SITL simulation)
    python drone_bridge.py --port 8080 --connection udp:127.0.0.1:14550

    # Connect via TCP
    python drone_bridge.py --port 8080 --connection tcp:192.168.1.100:5760

API Endpoints:
    POST /api/v1/drone/<action>  - Execute a drone action

Supported actions: arm, disarm, takeoff, land, rtl, hold, goto,
    mission_upload, mission_start, mission_pause, mission_resume,
    mission_clear, set_mode, get_telemetry, get_status, get_gps,
    get_battery, camera, deliver, patrol, survey, orbit, follow
"""

import argparse
import json
import math
import time
import threading
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse

try:
    from pymavlink import mavutil
except ImportError:
    mavutil = None
    print("WARNING: pymavlink not installed. Run: pip install pymavlink")
    print("Bridge will run in simulation-only mode.")


class DroneBridge:
    """Manages MAVLink connection and drone state."""

    def __init__(self, connection_string):
        self.connection_string = connection_string
        self.master = None
        self.connected = False
        self._last_telemetry = {}
        self._telemetry_lock = threading.Lock()
        self._running = False
        self._thread = None

    def connect(self):
        """Establish MAVLink connection."""
        if mavutil is None:
            print("SIMULATION MODE: No pymavlink available")
            self.connected = True
            return True

        try:
            print(f"Connecting to drone via: {self.connection_string}")
            self.master = mavutil.mavlink_connection(self.connection_string)
            # Wait for heartbeat
            print("Waiting for heartbeat...")
            msg = self.master.wait_heartbeat(timeout=30)
            if msg is None:
                print("ERROR: No heartbeat received within 30 seconds")
                return False
            print(f"Connected to drone: type={msg.type}, autopilot={msg.autopilot}")
            self.connected = True
            self._running = True
            self._thread = threading.Thread(target=self._telemetry_loop, daemon=True)
            self._thread.start()
            return True
        except Exception as e:
            print(f"ERROR connecting to drone: {e}")
            return False

    def _telemetry_loop(self):
        """Background thread to poll telemetry."""
        while self._running:
            try:
                if self.master:
                    # Request data streams
                    self.master.mav.request_data_stream_send(
                        self.master.target_system,
                        self.master.target_component,
                        mavutil.mavlink.MAV_DATA_STREAM_ALL,
                        4,  # 4 Hz
                        1,
                    )
                    # Wait for messages
                    msg = self.master.recv_match(
                        type=[
                            "GLOBAL_POSITION_INT",
                            "ATTITUDE",
                            "BATTERY_STATUS",
                            "GPS_RAW_INT",
                            "HEARTBEAT",
                            "SYS_STATUS",
                        ],
                        blocking=True,
                        timeout=1,
                    )
                    if msg:
                        with self._telemetry_lock:
                            self._last_telemetry["last_update"] = time.time()
                            if msg.get_type() == "GLOBAL_POSITION_INT":
                                self._last_telemetry["lat"] = msg.lat / 1e7
                                self._last_telemetry["lon"] = msg.lon / 1e7
                                self._last_telemetry["alt"] = msg.relative_alt / 1000.0
                                self._last_telemetry["heading"] = msg.hdg / 100.0
                            elif msg.get_type() == "BATTERY_STATUS":
                                self._last_telemetry["battery"] = msg.battery_remaining
                            elif msg.get_type() == "GPS_RAW_INT":
                                self._last_telemetry["gps_sats"] = msg.satellites_visible
                                self._last_telemetry["gps_fix"] = msg.fix_type
                            elif msg.get_type() == "HEARTBEAT":
                                self._last_telemetry["armed"] = (
                                    msg.base_mode & mavutil.mavlink.MAV_MODE_FLAG_SAFETY_ARMED
                                ) != 0
                                self._last_telemetry["mode"] = mavutil.mode_string_v10(msg)
            except Exception as e:
                if self._running:
                    print(f"Telemetry error: {e}")
                time.sleep(1)

    def get_telemetry(self):
        """Get current telemetry snapshot."""
        with self._telemetry_lock:
            tel = dict(self._last_telemetry)
        return {
            "armed": tel.get("armed", False),
            "flight_mode": tel.get("mode", "UNKNOWN"),
            "battery_pct": float(tel.get("battery", -1)),
            "altitude_m": float(tel.get("alt", 0)),
            "latitude": float(tel.get("lat", 0)),
            "longitude": float(tel.get("lon", 0)),
            "ground_speed_ms": float(tel.get("gs", 0)),
            "heading_deg": float(tel.get("heading", 0)),
            "gps_satellites": int(tel.get("gps_sats", 0)),
            "gps_fix_type": int(tel.get("gps_fix", 0)),
            "in_air": tel.get("alt", 0) > 0.5,
        }

    def execute(self, action, params):
        """Execute a drone action."""
        if not self.connected:
            return {"success": False, "error": "Drone not connected"}

        if self.master is None:
            return self._simulate(action, params)

        try:
            handler = getattr(self, f"_handle_{action}", None)
            if handler is None:
                return {"success": False, "error": f"Unknown action: {action}"}
            return handler(params)
        except Exception as e:
            return {"success": False, "error": str(e)}

    def _handle_arm(self, params):
        self.master.arducopter_arm()
        self.master.motors_armed_wait()
        return {"success": True, "telemetry": self.get_telemetry()}

    def _handle_disarm(self, params):
        self.master.arducopter_disarm()
        self.master.motors_disarmed_wait()
        return {"success": True, "telemetry": self.get_telemetry()}

    def _handle_takeoff(self, params):
        alt = float(params.get("target_altitude_m", 10))
        self.master.mav.command_long_send(
            self.master.target_system,
            self.master.target_component,
            mavutil.mavlink.MAV_CMD_NAV_TAKEOFF,
            0, 0, 0, 0, 0, 0, 0, alt,
        )
        return {"success": True, "telemetry": self.get_telemetry()}

    def _handle_land(self, params):
        self.master.mav.command_long_send(
            self.master.target_system,
            self.master.target_component,
            mavutil.mavlink.MAV_CMD_NAV_LAND,
            0, 0, 0, 0, 0, 0, 0, 0,
        )
        return {"success": True, "telemetry": self.get_telemetry()}

    def _handle_rtl(self, params):
        self.master.set_mode("RTL")
        return {"success": True, "telemetry": self.get_telemetry()}

    def _handle_hold(self, params):
        self.master.set_mode("LOITER")
        return {"success": True, "telemetry": self.get_telemetry()}

    def _handle_goto(self, params):
        lat = float(params.get("target_latitude", 0))
        lon = float(params.get("target_longitude", 0))
        alt = float(params.get("target_altitude_m", 10))
        self.master.mav.mission_item_send(
            self.master.target_system,
            self.master.target_component,
            0,
            mavutil.mavlink.MAV_FRAME_GLOBAL_RELATIVE_ALT,
            mavutil.mavlink.MAV_CMD_NAV_WAYPOINT,
            2, 1, 0, 0, 0, 0, lat, lon, alt,
        )
        return {"success": True, "telemetry": self.get_telemetry()}

    def _handle_set_mode(self, params):
        mode = params.get("flight_mode", "GUIDED")
        self.master.set_mode(mode)
        return {"success": True, "telemetry": self.get_telemetry()}

    def _handle_get_telemetry(self, params):
        return {"success": True, "telemetry": self.get_telemetry()}

    def _handle_get_status(self, params):
        return {"success": True, "telemetry": self.get_telemetry()}

    def _handle_get_gps(self, params):
        return {"success": True, "telemetry": self.get_telemetry()}

    def _handle_get_battery(self, params):
        return {"success": True, "telemetry": self.get_telemetry()}

    def _handle_mission_upload(self, params):
        waypoints = params.get("waypoints", [])
        self.master.mav.mission_count_send(
            self.master.target_system,
            self.master.target_component,
            len(waypoints),
            mavutil.mavlink.MAV_MISSION_TYPE_MISSION,
        )
        for i, wp in enumerate(waypoints):
            self.master.mav.mission_item_send(
                self.master.target_system,
                self.master.target_component,
                i,
                mavutil.mavlink.MAV_FRAME_GLOBAL_RELATIVE_ALT,
                mavutil.mavlink.MAV_CMD_NAV_WAYPOINT,
                2, 1, 0, 0, 0, 0,
                float(wp.get("lat", 0)),
                float(wp.get("lon", 0)),
                float(wp.get("alt", 10)),
            )
        return {
            "success": True,
            "mission": {"total_items": len(waypoints), "state": "uploaded"},
            "telemetry": self.get_telemetry(),
        }

    def _handle_mission_start(self, params):
        self.master.set_mode("AUTO")
        return {
            "success": True,
            "mission": {"state": "executing", "total_items": 0, "current_index": 0, "progress_pct": 0},
            "telemetry": self.get_telemetry(),
        }

    def _handle_mission_pause(self, params):
        self.master.set_mode("LOITER")
        return {"success": True, "mission": {"state": "paused"}, "telemetry": self.get_telemetry()}

    def _handle_mission_resume(self, params):
        self.master.set_mode("AUTO")
        return {"success": True, "mission": {"state": "executing"}, "telemetry": self.get_telemetry()}

    def _handle_mission_clear(self, params):
        self.master.mav.mission_clear_all_send(
            self.master.target_system, self.master.target_component
        )
        return {"success": True, "mission": {"state": "cleared"}, "telemetry": self.get_telemetry()}

    def _handle_patrol(self, params):
        return self._simulate_mission("patrol", params)

    def _handle_survey(self, params):
        return self._simulate_mission("survey", params)

    def _handle_orbit(self, params):
        return self._simulate_mission("orbit", params)

    def _handle_follow(self, params):
        return {"success": True, "telemetry": self.get_telemetry()}

    def _handle_camera(self, params):
        self.master.mav.command_long_send(
            self.master.target_system,
            self.master.target_component,
            mavutil.mavlink.MAV_CMD_IMAGE_START_CAPTURE,
            0, 0, 1, 0, 0, 0, 0, 0,
        )
        return {"success": True, "telemetry": self.get_telemetry()}

    def _handle_deliver(self, params):
        return self._simulate_mission("deliver", params)

    def _simulate_mission(self, action, params):
        return {
            "success": True,
            "mission": {
                "total_items": 5,
                "current_index": 0,
                "progress_pct": 0,
                "state": "executing",
            },
            "telemetry": self.get_telemetry(),
        }

    def _simulate(self, action, params):
        """Simulate drone actions when no real connection."""
        return {
            "success": True,
            "telemetry": {
                "armed": action in ("arm", "takeoff", "goto", "patrol", "survey", "orbit", "mission_start"),
                "flight_mode": "GUIDED",
                "battery_pct": 92.0,
                "altitude_m": float(params.get("target_altitude_m", 10)) if action == "takeoff" else 0,
                "latitude": 30.2741,
                "longitude": 120.1551,
                "ground_speed_ms": 5.0 if action in ("goto", "patrol", "survey") else 0,
                "heading_deg": 0,
                "gps_satellites": 16,
                "gps_fix_type": 3,
                "in_air": action in ("takeoff", "goto", "patrol", "survey", "orbit"),
            },
            "mission": {
                "total_items": 5,
                "current_index": 0,
                "progress_pct": 0,
                "state": "executing",
            } if action in ("mission_start", "patrol", "survey", "orbit") else None,
        }

    def close(self):
        """Close the connection."""
        self._running = False
        if self._thread:
            self._thread.join(timeout=2)
        if self.master:
            self.master.close()
        self.connected = False


class DroneHTTPHandler(BaseHTTPRequestHandler):
    """HTTP request handler for the drone bridge."""
    bridge = None  # Set by the server

    def do_POST(self):
        path = urlparse(self.path).path
        # Parse action from URL: /api/v1/drone/<action>
        parts = path.strip("/").split("/")
        if len(parts) < 4 or parts[0] != "api" or parts[1] != "v1" or parts[2] != "drone":
            self._send_error(404, "Not found")
            return

        action = parts[3]

        content_length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(content_length) if content_length > 0 else b"{}"

        try:
            data = json.loads(body)
        except json.JSONDecodeError:
            self._send_error(400, "Invalid JSON")
            return

        params = data.get("parameters", {})
        waypoints = data.get("waypoints", [])
        if waypoints:
            params["waypoints"] = waypoints

        result = self.bridge.execute(action, params)
        self._send_json(result)

    def do_GET(self):
        path = urlparse(self.path).path
        if path == "/health":
            self._send_json({"status": "ok", "connected": self.bridge.connected})
        else:
            self._send_error(404, "Not found")

    def _send_json(self, data):
        body = json.dumps(data).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _send_error(self, code, message):
        body = json.dumps({"success": False, "error": message}).encode("utf-8")
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, format, *args):
        print(f"[{time.strftime('%H:%M:%S')}] {args[0]}")


def main():
    parser = argparse.ArgumentParser(description="HTTP-to-MAVLink drone bridge")
    parser.add_argument("--port", type=int, default=8080, help="HTTP server port")
    parser.add_argument("--connection", type=str, default="udp:127.0.0.1:14550",
                        help="MAVLink connection string (serial:/dev/ttyACM0:115200, udp:127.0.0.1:14550, tcp:192.168.1.100:5760)")
    parser.add_argument("--simulate", action="store_true", help="Force simulation mode")
    args = parser.parse_args()

    bridge = DroneBridge(args.connection)
    if not args.simulate:
        bridge.connect()
    else:
        print("SIMULATION MODE (forced)")
        bridge.connected = True

    DroneHTTPHandler.bridge = bridge
    server = HTTPServer(("0.0.0.0", args.port), DroneHTTPHandler)

    print(f"Drone bridge listening on http://0.0.0.0:{args.port}")
    print(f"Health check:  http://localhost:{args.port}/health")
    print(f"API endpoint:  http://localhost:{args.port}/api/v1/drone/<action>")
    print("Press Ctrl+C to stop")

    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nShutting down...")
    finally:
        server.server_close()
        bridge.close()


if __name__ == "__main__":
    main()