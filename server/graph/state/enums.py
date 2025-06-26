"""
Enumerations for PaladinAI LangGraph Workflow State Management.

This module defines the enums used throughout the workflow system
for categorization and complexity assessment.
"""

from enum import Enum


class WorkflowType(str, Enum):
    """
    Enumeration of supported workflow types.
    
    These represent the three main categories of user requests
    that the system can handle.
    """
    QUERY = "QUERY"
    INCIDENT = "INCIDENT" 
    ACTION = "ACTION"
    
    @classmethod
    def get_description(cls, workflow_type: str) -> str:
        """
        Get a human-readable description of the workflow type.
        
        Args:
            workflow_type: The workflow type to describe
            
        Returns:
            Description string for the workflow type
        """
        descriptions = {
            cls.QUERY: "Quick status/boolean information requests",
            cls.INCIDENT: "Problem investigation and root cause analysis",
            cls.ACTION: "Data retrieval, analysis, and reporting tasks"
        }
        return descriptions.get(workflow_type, "Unknown workflow type")
    
    @classmethod
    def get_typical_response_time(cls, workflow_type: str) -> str:
        """
        Get typical response time for a workflow type.
        
        Args:
            workflow_type: The workflow type
            
        Returns:
            Typical response time string
        """
        response_times = {
            cls.QUERY: "< 30 seconds",
            cls.INCIDENT: "2-10 minutes", 
            cls.ACTION: "1-5 minutes"
        }
        return response_times.get(workflow_type, "Variable")
    
    @classmethod
    def get_data_sources(cls, workflow_type: str) -> list[str]:
        """
        Get typical data sources for a workflow type.
        
        Args:
            workflow_type: The workflow type
            
        Returns:
            List of typical data sources
        """
        data_sources = {
            cls.QUERY: ["Prometheus metrics", "Alertmanager alerts"],
            cls.INCIDENT: ["Prometheus metrics", "Loki logs", "Alertmanager alerts"],
            cls.ACTION: ["Historical metrics", "Log aggregations", "Performance data"]
        }
        return data_sources.get(workflow_type, ["To be determined"])


class ComplexityLevel(str, Enum):
    """
    Enumeration of workflow complexity levels.
    
    These represent the estimated complexity and resource requirements
    for processing different types of requests.
    """
    LOW = "LOW"
    MEDIUM = "MEDIUM"
    HIGH = "HIGH"
    
    @classmethod
    def get_description(cls, complexity: str) -> str:
        """
        Get a description of the complexity level.
        
        Args:
            complexity: The complexity level
            
        Returns:
            Description string for the complexity level
        """
        descriptions = {
            cls.LOW: "Simple, straightforward requests requiring minimal processing",
            cls.MEDIUM: "Moderate complexity requests requiring some analysis",
            cls.HIGH: "Complex requests requiring extensive processing and analysis"
        }
        return descriptions.get(complexity, "Unknown complexity level")
    
    @classmethod
    def get_estimated_resources(cls, complexity: str) -> dict[str, str]:
        """
        Get estimated resource requirements for a complexity level.
        
        Args:
            complexity: The complexity level
            
        Returns:
            Dictionary with resource estimates
        """
        resources = {
            cls.LOW: {
                "cpu": "Low",
                "memory": "Low", 
                "api_calls": "1-2",
                "processing_time": "< 10 seconds"
            },
            cls.MEDIUM: {
                "cpu": "Medium",
                "memory": "Medium",
                "api_calls": "3-5", 
                "processing_time": "10-60 seconds"
            },
            cls.HIGH: {
                "cpu": "High",
                "memory": "High",
                "api_calls": "5+",
                "processing_time": "> 60 seconds"
            }
        }
        return resources.get(complexity, {
            "cpu": "Unknown",
            "memory": "Unknown",
            "api_calls": "Unknown",
            "processing_time": "Unknown"
        })


class NodeStatus(str, Enum):
    """
    Enumeration of node execution statuses.
    
    These represent the current state of individual nodes
    during workflow execution.
    """
    PENDING = "PENDING"
    RUNNING = "RUNNING"
    COMPLETED = "COMPLETED"
    FAILED = "FAILED"
    SKIPPED = "SKIPPED"
    
    @classmethod
    def is_terminal(cls, status: str) -> bool:
        """
        Check if a status represents a terminal state.
        
        Args:
            status: The node status
            
        Returns:
            True if the status is terminal (no further processing)
        """
        terminal_statuses = {cls.COMPLETED, cls.FAILED, cls.SKIPPED}
        return status in terminal_statuses
    
    @classmethod
    def is_error(cls, status: str) -> bool:
        """
        Check if a status represents an error state.
        
        Args:
            status: The node status
            
        Returns:
            True if the status indicates an error
        """
        return status == cls.FAILED


class ExecutionPhase(str, Enum):
    """
    Enumeration of workflow execution phases.
    
    These represent the high-level phases of workflow execution
    for tracking and monitoring purposes.
    """
    INITIALIZATION = "INITIALIZATION"
    CATEGORIZATION = "CATEGORIZATION"
    PROCESSING = "PROCESSING"
    FINALIZATION = "FINALIZATION"
    COMPLETED = "COMPLETED"
    ERROR = "ERROR"
    
    @classmethod
    def get_next_phase(cls, current_phase: str) -> str:
        """
        Get the next logical phase in the workflow.
        
        Args:
            current_phase: The current execution phase
            
        Returns:
            The next phase in the workflow
        """
        phase_order = [
            cls.INITIALIZATION,
            cls.CATEGORIZATION, 
            cls.PROCESSING,
            cls.FINALIZATION,
            cls.COMPLETED
        ]
        
        try:
            current_index = phase_order.index(current_phase)
            if current_index < len(phase_order) - 1:
                return phase_order[current_index + 1]
            else:
                return cls.COMPLETED
        except ValueError:
            return cls.ERROR
    
    @classmethod
    def is_complete(cls, phase: str) -> bool:
        """
        Check if a phase represents completion.
        
        Args:
            phase: The execution phase
            
        Returns:
            True if the phase indicates completion
        """
        return phase in {cls.COMPLETED, cls.ERROR}
