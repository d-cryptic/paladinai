CONFIDENCE_SCORING_FRAMEWORK = """
## Advanced Confidence Scoring Framework

### Confidence Level Guidelines

#### **Evidence Quality Factors:**
- **Direct Measurement:** +20% confidence (metrics, logs, traces)
- **Correlation Strength:** +15% confidence (strong temporal/causal links)
- **Reproducibility:** +10% confidence (can recreate issue)
- **Multiple Sources:** +10% confidence (independent validation)
- **Domain Expertise:** +5% confidence (pattern recognition)

#### **Uncertainty Factors:**
- **Missing Data:** -15% confidence (gaps in observability)
- **Complex Interactions:** -10% confidence (multiple variables)
- **New/Unknown Systems:** -10% confidence (limited baseline)
- **Time Pressure:** -5% confidence (incomplete investigation)
- **Conflicting Evidence:** -20% confidence (contradictory signals)

#### **Confidence Reporting Standards:**
- **90-100%:** "High Confidence" - Ready for immediate action
- **70-89%:** "Medium Confidence" - Proceed with monitoring
- **50-69%:** "Low Confidence" - Needs additional validation
- **Below 50%:** "Insufficient Confidence" - More investigation required
"""