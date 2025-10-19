---
theme: default
title: Paladin-AI
info: |
  ## Paladin-AI
  An Agentic Assistant for Infrastructure Debugging and Analysis

  By Barun Debnath
class: text-center
drawings:
  persist: false
transition: slide-left
mdc: true
css: unocss
---

<style>
.slidev-layout {
  background: linear-gradient(135deg, #ffeee6 0%, #ffe4d6 100%) !important;
}

.dark .slidev-layout {
  background: linear-gradient(135deg, #2d1810 0%, #3d2218 100%) !important;
}

/* Ensure text remains readable on light peach background */
.slidev-layout h1, .slidev-layout h2, .slidev-layout h3 {
  color: #2d1810 !important;
}

.slidev-layout p, .slidev-layout li {
  color: #3d2218 !important;
}

/* Adjust code blocks for better contrast */
.slidev-layout pre {
  background: rgba(45, 24, 16, 0.1) !important;
  border: 1px solid rgba(45, 24, 16, 0.2) !important;
}
</style>

---

# Paladin-AI
## An Agentic Assistant for Infrastructure Debugging and Analysis

**By: Barun Debnath**

```
    ____        __          ___          ___    ____
   / __ \____ _/ /___ _____/ (_)___     /   |  /  _/
  / /_/ / __ `/ / __ `/ __  / / __ \   / /| |  / /
 / ____/ /_/ / / /_/ / /_/ / / / / /  / ___ |_/ /
/_/    \__,_/_/\__,_/\__,_/_/_/ /_/  /_/  |_/___/
```

### Key Features

- 🤖 **Agentic Assistant** - AI-powered debugging and analysis
- 🌐 **Multi-Platform** - CLI, API, Web app, Discord bot
- 🔗 **Integrations** - Prometheus, Alertmanager, and Loki

<!--
Hello Everyone,
I am Barun Debnath,
And I am presenting my product "Paladin AI"

Paladin-AI is an intelligent infrastructure assistant that helps SREs and DevOps teams debug and analyze infrastructure issues using AI-powered workflows.
-->

---

# Demo

## 💬 Chat App
- Query, Action and Incident Flow
- Memory/instructions and Documentation as context

## 🎮 Discord MCP
- Auto Alert Analysis
- Chat and discussions as memory
- Discord Bot

## 🕸️ Neo4j Memory Graph
- Dynamic relationship mapping

<!--
Without further ado, let's move onto demo.

just to inform, I have created a mock infra in background (3 tier architecture with other dbs/queue/observability etc.)

Paladin AI categorizes any user input into three types, query, action, incident.
Lets first see query type input:
Now lets see action type input:
now finally the incident input

Let's see other features:
1. Memory or instructions
you can add memory to do analysis in your own way - you want x metrics along with y or you want to ignore any service's logs while doing analysis. etc.
Memory are fetched everytime for every query

2. Document 
You can add pdf or markdown files and it will be added in our vector DB. 
Everytime during alert auto analysis (more on this later) these docs will be fetched to do in depth analysis

Now let's move on to discord to see the functionalities there
1. Discord Bot
- tag the bot and ask for any query/action

2. Store all conversation as memory. This stores relevant conversation in Paladin's memory so that it can learn/react in realtime

3. Upload docs from discord itself

4. Auto alert analysis in almost realtime, It provides a detailed doc for this. 

Now quickly let me show you how we store the memory
Every memory is store in graph format with all relevant previous and new memories
Paladin fetch 1st and 2nd degree related memories for each user query
-->

---
layout: default
---

# How is it useful for SRE?

### ⚡ Faster Incident Response
Automatically analyzes alerts and logs to provide immediate insights, reducing MTTR from hours to minutes through intelligent correlation of metrics and events.

### 🧠 Intelligent Context Retention
Learns from past incidents and debugging sessions, building a knowledge graph that helps identify patterns and suggest solutions based on historical data.

### 🔄 Automated Workflow Orchestration
Streamlines complex debugging processes by automatically selecting appropriate tools and executing multi-step analysis workflows based on query type and context.

### 🤝 Enhanced Team Collaboration
Integrates with Discord and chat platforms to provide real-time assistance during incidents, enabling seamless knowledge sharing across team members.

<!--
Through Paladin AI, SREs can
1. do faster and. indepth incident anlaysis and respond to it with more context
2. Alert analysis is automated hence decreasing the time taken to react to an alert
3. As it integrates in discord (in future slack too), everyone can ask for real time assistant
-->

---
layout: default
---

# Future Scope - Core Enhancements

### 🚀 Performance Optimization
Decreasing the latency in tool calling and enhancing accuracy through optimized LLM inference, caching strategies, and parallel processing workflows.

### 🔌 MCP Server Integration Engine
Create a universal engine to integrate other MCP servers, enabling seamless connectivity with diverse monitoring and observability tools across different platforms.

### 👤 Learning-Based User Authentication
Implement user authentication with debugging process tracking, enabling the LLM to learn from senior employees' debugging patterns and mentor junior team members.

<!--
In future,
Paladin AI will be able to do analysis in seconds by parallellizing the tool and api calls
MCP server engine which can integrate with almost all SRE related mcps like aws cost, github etc
Focusing on self learning from the way experienced people debug the alert/incident using paladin and through this learning guide the juniors/new folks through suggestions.
-->

---
layout: center
class: text-center
---

# Thank You

<div class="mt-8 text-lg">
<span class="text-purple-400">Paladin-AI</span> - Your Infrastructure Guardian
</div>

<!--
That's all.
Thank you very much for listening!!
-->
