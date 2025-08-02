"""
Prompts module for Discord MCP
"""
from .guardrail_prompts import PALADIN_GUARDRAIL_SYSTEM_PROMPT, PALADIN_KEYWORDS
from .discord_bot_prompts import (
    ACKNOWLEDGMENT_SYSTEM_PROMPT,
    DISCORD_FORMATTING_SYSTEM_PROMPT,
    get_acknowledgment_user_prompt,
    get_formatting_user_prompt
)

__all__ = [
    'PALADIN_GUARDRAIL_SYSTEM_PROMPT', 
    'PALADIN_KEYWORDS',
    'ACKNOWLEDGMENT_SYSTEM_PROMPT',
    'DISCORD_FORMATTING_SYSTEM_PROMPT',
    'get_acknowledgment_user_prompt',
    'get_formatting_user_prompt'
]