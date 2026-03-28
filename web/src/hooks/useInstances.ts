import { useEffect, useRef, useState } from "react";
import { containerClient } from "../client";
import type { Container } from "../gen/norn/containers/v1/containers_pb";

export function useInstances(intervalMs = 3000) {
  const [instances, setInstances] = useState<Container[]>([]);
  const [loading, setLoading] = useState(true);
  const firstLoad = useRef(true);

  useEffect(() => {
    let active = true;
    const poll = async () => {
      try {
        const res = await containerClient.listContainers({});
        if (active) {
          setInstances(res.containers);
          if (firstLoad.current) { firstLoad.current = false; setLoading(false); }
        }
      } catch {
        if (active && firstLoad.current) { firstLoad.current = false; setLoading(false); }
      }
    };
    poll();
    const id = setInterval(poll, intervalMs);
    return () => { active = false; clearInterval(id); };
  }, [intervalMs]);

  return { instances, loading };
}
