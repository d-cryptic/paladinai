"""Robust JSON parser for LLM responses"""

import json
import re
from typing import Any, Dict, Optional


def parse_llm_json(content: str) -> Dict[str, Any]:
    """
    Parse JSON from LLM response with various fallbacks.
    
    Args:
        content: Raw LLM response content
        
    Returns:
        Parsed JSON as dictionary
        
    Raises:
        json.JSONDecodeError: If all parsing attempts fail
    """
    content = content.strip()
    
    # Try direct parsing first
    try:
        return json.loads(content)
    except json.JSONDecodeError:
        pass
    
    # Handle markdown code blocks
    if "```json" in content:
        try:
            json_str = content.split("```json")[1].split("```")[0].strip()
            return json.loads(json_str)
        except (IndexError, json.JSONDecodeError):
            pass
    
    if "```" in content:
        try:
            json_str = content.split("```")[1].split("```")[0].strip()
            return json.loads(json_str)
        except (IndexError, json.JSONDecodeError):
            pass
    
    # Try to find JSON object in the content
    # Look for content between { and }
    json_match = re.search(r'\{[^{}]*\{.*\}[^{}]*\}|\{[^{}]*\}', content, re.DOTALL)
    if json_match:
        try:
            return json.loads(json_match.group(0))
        except json.JSONDecodeError:
            pass
    
    # Handle incomplete JSON (missing opening brace)
    if content.startswith('"') or content.lstrip().startswith('"'):
        try:
            # Try adding opening brace
            fixed_json = "{" + content
            return json.loads(fixed_json)
        except json.JSONDecodeError:
            pass
    
    # Handle JSON that starts after some text
    lines = content.split('\n')
    for i, line in enumerate(lines):
        if line.strip().startswith('{'):
            try:
                json_str = '\n'.join(lines[i:])
                return json.loads(json_str)
            except json.JSONDecodeError:
                pass
    
    # Last resort: try to extract any valid JSON from the content
    # This handles cases where JSON might be embedded in text
    try:
        # Find all potential JSON objects
        potential_jsons = re.findall(r'\{(?:[^{}]|(?:\{[^{}]*\}))*\}', content)
        for potential_json in potential_jsons:
            try:
                return json.loads(potential_json)
            except json.JSONDecodeError:
                continue
    except:
        pass
    
    # If all else fails, raise the original error
    raise json.JSONDecodeError(f"Could not parse JSON from content: {content[:200]}...", content, 0)