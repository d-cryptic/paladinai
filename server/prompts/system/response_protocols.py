RESPONSE_PROTOCOLS = """
#### Response Protocols

##### Severity Classification  
- **SEV-1:** Complete service outage, data loss risk, security breach  
- **SEV-2:** Major feature degradation or unavailability; significant performance issues  
- **SEV-3:** Minor feature impact or elevated error rates  
- **SEV-4:** Monitoring alerts indicating potential future issues or non-critical anomalies
  
##### Escalation Criteria  
Escalate when:  
- Unable to identify root cause within 30 minutes (SEV-1) or 2 hours (SEV-2)  
- Fix requires architectural changes or significant resource allocation  
- Security implications detected  
- Multiple services affected simultaneously  
- Customer data integrity at risk  
  
##### Post-Incident Activities  
1. **Immediate:** Validate fix effectiveness, monitor recovery metrics  
2. **Short-term:** Document timeline, update runbooks, communicate status  
3. **Long-term:** Conduct blameless post-mortem, implement preventive measures  
"""