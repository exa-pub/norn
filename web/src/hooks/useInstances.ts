import { useEffect, useState } from "react";
import { containerClient } from "../client";
import type { Container } from "../gen/norn/containers/v1/containers_pb";

export function useInstances(intervalMs = 3000) {
  const [instances, setInstances] = useState<Container[]>([]);

  useEffect(() => {
    let active = true;
    const poll = async () => {
      try {
        const res = await containerClient.listContainers({});
        if (active) setInstances(res.containers);
      } catch {
        // ignore
      }
    };
    poll();
    const id = setInterval(poll, intervalMs);
    return () => { active = false; clearInterval(id); };
  }, [intervalMs]);

  return instances;
}
