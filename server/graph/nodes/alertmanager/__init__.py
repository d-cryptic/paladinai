"""
Alertmanager node for PaladinAI workflow.

This module handles alert data collection and analysis.
"""

from .alertmanager import AlertmanagerNode, alertmanager_node

__all__ = ["AlertmanagerNode", "alertmanager_node"]