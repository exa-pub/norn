import { useEffect, useState } from "react";
import { terminalClient } from "../client";
import type { Terminal } from "../gen/norn/terminals/v1/terminals_pb";

export function useTerminals(instanceName: string | null, intervalMs = 3000) {
  const [terminals, setTerminals] = useState<Terminal[]>([]);

  useEffect(() => {
    if (!instanceName) { setTerminals([]); return; }
    let active = true;
    const poll = async () => {
      try {
        const res = await terminalClient.listTerminals({ instanceName });
        if (active) setTerminals(res.terminals);
      } catch {
        if (active) setTerminals([]);
      }
    };
    poll();
    const id = setInterval(poll, intervalMs);
    return () => { active = false; clearInterval(id); };
  }, [instanceName, intervalMs]);

  return terminals;
}
