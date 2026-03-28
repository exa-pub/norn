import { useEffect, useState } from "react";
import { agentClient } from "../client";
import type { AgentSession } from "../gen/norn/agents/v1/agents_pb";

export function useAgents(instanceName: string | null, intervalMs = 3000) {
  const [agents, setAgents] = useState<AgentSession[]>([]);

  useEffect(() => {
    if (!instanceName) { setAgents([]); return; }
    let active = true;
    const poll = async () => {
      try {
        const res = await agentClient.listAgentSessions({ instanceName });
        if (active) setAgents(res.sessions);
      } catch {
        if (active) setAgents([]);
      }
    };
    poll();
    const id = setInterval(poll, intervalMs);
    return () => { active = false; clearInterval(id); };
  }, [instanceName, intervalMs]);

  return agents;
}
