"""
Loading screen utilities for Paladin AI CLI
Provides animated loading indicators for async operations.
"""

import asyncio
import sys
from typing import Optional


class LoadingSpinner:
    """Animated loading spinner for CLI operations."""
    
    def __init__(self, message: str = "Processing", spinner_chars: str = "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏"):
        """
        Initialize the loading spinner.
        
        Args:
            message: Message to display alongside spinner
            spinner_chars: Characters to cycle through for animation
        """
        self.message = message
        self.spinner_chars = spinner_chars
        self.is_spinning = False
        self._task: Optional[asyncio.Task] = None
    
    async def start(self) -> None:
        """Start the loading spinner animation."""
        if self.is_spinning:
            return
            
        self.is_spinning = True
        self._task = asyncio.create_task(self._spin())
    
    async def stop(self) -> None:
        """Stop the loading spinner animation."""
        if not self.is_spinning:
            return
            
        self.is_spinning = False
        if self._task:
            self._task.cancel()
            try:
                await self._task
            except asyncio.CancelledError:
                pass
        
        # Clear the spinner line
        sys.stdout.write('\r' + ' ' * (len(self.message) + 10) + '\r')
        sys.stdout.flush()
    
    async def _spin(self) -> None:
        """Internal method to handle the spinning animation."""
        try:
            idx = 0
            while self.is_spinning:
                char = self.spinner_chars[idx % len(self.spinner_chars)]
                sys.stdout.write(f'\r{char} {self.message}...')
                sys.stdout.flush()
                idx += 1
                await asyncio.sleep(0.1)
        except asyncio.CancelledError:
            pass


async def with_loading(coro, message: str = "Processing"):
    """
    Execute a coroutine with a loading spinner.

    Args:
        coro: Coroutine to execute
        message: Loading message to display

    Returns:
        Result of the coroutine
    """
    spinner = LoadingSpinner(message)
    try:
        await spinner.start()
        result = await coro
        return result
    finally:
        await spinner.stop()
