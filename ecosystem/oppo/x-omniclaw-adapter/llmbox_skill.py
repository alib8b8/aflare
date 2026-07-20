"""
llm-box X-OmniClaw Skill Adapter
Bridges llm-box workflow engine into X-OmniClaw as a Skill layer.
"""

import json
import subprocess
import os
from typing import Dict, Any, Optional


class LLMBoxSkillAdapter:
    """
    X-OmniClaw Skill that delegates to llm-box workflow engine.
    Implements the X-OmniClaw Skill interface: perceive → plan → execute.
    """

    def __init__(self, llm_box_path: str = "llm-box", default_model: str = "qwen2-1.5b"):
        self.llm_box_path = llm_box_path
        self.default_model = default_model
        self.power_profile = "balanced"

    def perceive(self, context: Dict[str, Any]) -> Dict[str, Any]:
        """
        Extract user intent from X-OmniClaw perception context.

        Args:
            context: X-OmniClaw perception data containing:
                - screen_text: OCR text from current screen
                - voice_text: Voice recognition result
                - camera_objects: Detected objects from camera
                - active_app: Current foreground app package name
                - user_location: GPS coordinates

        Returns:
            Intent dict with type, parameters, and suggested workflow.
        """
        voice_text = context.get("voice_text", "")
        screen_text = context.get("screen_text", "")
        active_app = context.get("active_app", "")
        camera_objects = context.get("camera_objects", [])

        # Combine available text inputs
        user_input = voice_text or screen_text
        if not user_input and camera_objects:
            user_input = f"识别到物体: {', '.join(camera_objects)}"

        if not user_input:
            return {"intent": "idle", "confidence": 0.0, "workflow": None}

        # Use llm-box ondevice_llm to understand intent
        result = self._run_node("ondevice_llm", {
            "model": self.default_model,
            "quantization": "int4",
            "max_tokens": "200",
            "system_prompt": "你是一个意图理解助手。从用户输入中提取：1.意图类型 2.关键参数 3.推荐的工作流模板。输出JSON格式。",
        }, user_input)

        return {
            "intent": result,
            "input": user_input,
            "active_app": active_app,
            "confidence": 0.85,
        }

    def plan(self, intent: Dict[str, Any], memory: Optional[Dict] = None) -> Dict[str, Any]:
        """
        Map intent to llm-box workflow.

        Args:
            intent: Output from perceive()
            memory: PersonaX long-term memory

        Returns:
            Workflow specification ready for execution.
        """
        user_input = intent.get("input", "")
        if not user_input:
            return {"workflow": None, "error": "no input"}

        # Create workflow using llm-box
        result = self._run_command([
            self.llm_box_path, "create", user_input,
            "--platform", "mobile",
            "--offline",
            "--output", "/tmp/omniclaw-workflow.yaml"
        ])

        return {
            "workflow": "/tmp/omniclaw-workflow.yaml",
            "intent": intent,
            "memory_used": memory is not None,
            "plan_result": result,
        }

    def execute(self, workflow: Dict[str, Any], context: Dict[str, Any]) -> Dict[str, Any]:
        """
        Execute workflow with power awareness.

        Args:
            workflow: Output from plan()
            context: Current device context

        Returns:
            Execution result with X-OmniClaw-compatible actions.
        """
        # Check power state
        power = self._run_node("power_manager", {
            "profile": self.power_profile,
            "adaptive_mode": "true",
            "battery_aware": "true",
            "thermal_aware": "true",
        })

        effective_profile = power.get("effective_profile", "balanced")

        # Execute workflow
        workflow_path = workflow.get("workflow")
        if not workflow_path:
            return {"actions": [], "error": "no workflow"}

        result = self._run_command([
            self.llm_box_path, "run", workflow_path,
            "--power-profile", effective_profile,
        ])

        # Convert to X-OmniClaw action format
        actions = self._to_omniclaw_actions(result)

        # Audit on blockchain
        self._run_node("blockchain_audit", {
            "chain_type": "simulated",
            "audit_level": "workflow",
            "workflow_id": f"omniclaw_{int(time.time())}",
        })

        return {
            "actions": actions,
            "power_profile": effective_profile,
            "result": result,
        }

    def _run_node(self, node_name: str, params: Dict[str, str],
                  input_text: str = "") -> Dict[str, Any]:
        """Run a single llm-box node."""
        args = [self.llm_box_path, "node", node_name]
        for k, v in params.items():
            args.extend([f"--{k}", str(v)])
        if input_text:
            args.append(input_text)

        try:
            result = subprocess.run(
                args, capture_output=True, text=True, timeout=30,
                input=input_text if not input_text else None
            )
            if result.returncode == 0:
                return json.loads(result.stdout)
            return {"error": result.stderr}
        except (subprocess.TimeoutExpired, json.JSONDecodeError) as e:
            return {"error": str(e)}

    def _run_command(self, args: list) -> str:
        """Run a shell command."""
        try:
            result = subprocess.run(args, capture_output=True, text=True, timeout=60)
            return result.stdout if result.returncode == 0 else result.stderr
        except subprocess.TimeoutExpired:
            return "timeout"

    def _to_omniclaw_actions(self, result: str) -> list:
        """Convert llm-box result to X-OmniClaw action format."""
        actions = []
        try:
            data = json.loads(result) if isinstance(result, str) else result
            # Extract actionable items from result
            if isinstance(data, dict):
                if "interaction_plan" in data:
                    plan = data["interaction_plan"]
                    for step in plan.get("steps", []):
                        actions.append({
                            "type": step.get("action", "tap"),
                            "target": step.get("target", ""),
                            "coordinates": step.get("coordinates", {}),
                        })
                elif "response" in data:
                    actions.append({
                        "type": "speak",
                        "text": data["response"],
                    })
        except (json.JSONDecodeError, AttributeError):
            actions.append({"type": "speak", "text": str(result)[:200]})

        return actions


import time


# X-OmniClaw Skill entry point
def create_skill(config: Dict[str, Any] = None) -> LLMBoxSkillAdapter:
    """Create and configure the llm-box skill for X-OmniClaw."""
    config = config or {}
    return LLMBoxSkillAdapter(
        llm_box_path=config.get("llm_box_path", "llm-box"),
        default_model=config.get("default_model", "qwen2-1.5b"),
    )
