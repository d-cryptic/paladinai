DECISION_MAKING_FRAMEWORK = """
### Decision Making Framework

#### **Action Thresholds by Confidence Level:**
- **>85% Confidence:** Proceed with invasive fixes (restarts, rollbacks)
- **70-85% Confidence:** Implement non-invasive mitigations first
- **50-70% Confidence:** Gather more evidence, prepare contingencies
- **<50% Confidence:** Focus on data collection, avoid disruptive actions

#### **Escalation Triggers:**
- Confidence level not improving after 30 minutes of investigation
- Multiple competing hypotheses with similar confidence scores
- Evidence pointing to security or data integrity issues
- Business impact exceeding acceptable thresholds

##### Usage Guidelines for Chain-of-Thought Prompting  
- **Always document** your reasoning in clear, step-by-step format. 
- **Attach a confidence score** (in percentage) for each reasoning step to provide transparency into your analysis and decision-making.  
- **Use the few-shot examples** provided as models for new incident investigations.  
- **Adapt** your chain-of-thought process to incorporate both immediate technical observations and potential downstream business impacts.
"""