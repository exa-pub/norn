import { Box } from "@mantine/core";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Sidebar } from "./components/Sidebar/Sidebar";
import { AgentTabs } from "./components/AgentTabs/AgentTabs";
import { BottomPanel } from "./components/BottomPanel/BottomPanel";
import { StatusBar } from "./components/StatusBar";
import { CreateInstanceModal } from "./components/Modals/CreateInstance";
import { CreateAgentModal } from "./components/Modals/CreateAgent";
import { useInstances } from "./hooks/useInstances";
import { useAgents } from "./hooks/useAgents";
import { useTerminals } from "./hooks/useTerminals";
import { agentClient, containerClient, terminalClient } from "./client";
import { ContainerStatus } from "./gen/norn/containers/v1/containers_pb";
import type { AgentSession } from "./gen/norn/agents/v1/agents_pb";
import type { StreamLogsResponse } from "./gen/norn/containers/v1/containers_pb";

const MIN_SIDEBAR = 180;
const MAX_SIDEBAR = 500;
const MIN_BOTTOM = 80;
const MAX_BOTTOM_RATIO = 0.7;

export function App() {
  const instances = useInstances();
  const [selectedInstance, setSelectedInstance] = useState<string | null>(null);
  const agents = useAgents(selectedInstance);
  const terminals = useTerminals(selectedInstance);

  // Per-instance open tabs
  const [openAgentTabs, setOpenAgentTabs] = useState<Map<string, string[]>>(new Map());
  const [activeAgentTab, setActiveAgentTab] = useState<Map<string, string>>(new Map());
  const [activeTerminal, setActiveTerminal] = useState<string | null>(null);

  // Loading states
  const [launchingAgents, setLaunchingAgents] = useState<Set<string>>(new Set());
  const [creatingTerminal, setCreatingTerminal] = useState(false);

  // Modals
  const [createInstanceOpen, setCreateInstanceOpen] = useState(false);
  const [createAgentFor, setCreateAgentFor] = useState<string | null>(null);

  // Resizable panels
  const [sidebarWidth, setSidebarWidth] = useState(260);
  const [bottomHeight, setBottomHeight] = useState(300);
  const mainRef = useRef<HTMLDivElement>(null);

  // Bottom panel tabs (terminals | logs)
  const [activeBottomTab, setActiveBottomTab] = useState("terminals");

  // StreamLogs
  const [logs, setLogs] = useState<StreamLogsResponse[]>([]);
  const [logsActive, setLogsActive] = useState(false);
  const logsAbortRef = useRef<AbortController | null>(null);

  // Agents map for sidebar
  const agentsMap = useMemo(() => {
    const m = new Map<string, AgentSession[]>();
    if (selectedInstance) m.set(selectedInstance, agents);
    return m;
  }, [selectedInstance, agents]);

  const currentOpenTabs = selectedInstance ? (openAgentTabs.get(selectedInstance) ?? []) : [];
  const currentActiveTab = selectedInstance ? (activeAgentTab.get(selectedInstance) ?? null) : null;
  const liveAgentCount = agents.filter((a) => a.running).length;

  // Resolve active agent name for status bar
  const activeAgentName = useMemo(() => {
    if (!currentActiveTab) return null;
    const agent = agents.find((a) => a.id === currentActiveTab);
    return agent?.name || currentActiveTab.slice(0, 8);
  }, [currentActiveTab, agents]);

  // Auto-select first terminal
  useEffect(() => {
    if (terminals.length > 0 && !activeTerminal) {
      setActiveTerminal(terminals[0].id);
    }
  }, [terminals, activeTerminal]);

  // Prune stale tabs when agents disappear
  useEffect(() => {
    if (!selectedInstance) return;
    const agentIds = new Set(agents.map((a) => a.id));
    const tabs = openAgentTabs.get(selectedInstance) ?? [];
    const valid = tabs.filter((id) => agentIds.has(id));
    if (valid.length !== tabs.length) {
      setOpenAgentTabs((prev) => {
        const m = new Map(prev);
        m.set(selectedInstance, valid);
        return m;
      });
    }
  }, [agents, selectedInstance, openAgentTabs]);

  // StreamLogs for selected instance
  useEffect(() => {
    if (logsAbortRef.current) {
      logsAbortRef.current.abort();
      logsAbortRef.current = null;
    }
    setLogs([]);
    setLogsActive(false);

    if (!selectedInstance) return;
    const inst = instances.find((i) => i.name === selectedInstance);
    if (!inst || inst.status !== ContainerStatus.RUNNING) return;

    const abort = new AbortController();
    logsAbortRef.current = abort;
    setLogsActive(true);

    (async () => {
      try {
        const stream = containerClient.streamLogs(
          { name: selectedInstance },
          { signal: abort.signal },
        );
        for await (const entry of stream) {
          if (abort.signal.aborted) break;
          setLogs((prev) => {
            const next = [...prev, entry];
            // Keep last 5000 lines
            return next.length > 5000 ? next.slice(-5000) : next;
          });
        }
      } catch {
        // stream ended or aborted
      } finally {
        setLogsActive(false);
      }
    })();

    return () => abort.abort();
  }, [selectedInstance, instances]);

  // --- Auto-launch agent on tab open ---
  const autoLaunchedRef = useRef<Set<string>>(new Set());

  const openAgentTab = useCallback((instanceName: string, agentId: string) => {
    setSelectedInstance(instanceName);
    setOpenAgentTabs((prev) => {
      const m = new Map(prev);
      const tabs = m.get(instanceName) ?? [];
      if (!tabs.includes(agentId)) m.set(instanceName, [...tabs, agentId]);
      return m;
    });
    setActiveAgentTab((prev) => new Map(prev).set(instanceName, agentId));
  }, []);

  // Auto-launch effect: when an idle agent tab is opened and container is running
  useEffect(() => {
    if (!selectedInstance || !currentActiveTab) return;
    const inst = instances.find((i) => i.name === selectedInstance);
    if (!inst || inst.status !== ContainerStatus.RUNNING) return;

    const agent = agents.find((a) => a.id === currentActiveTab);
    if (!agent || agent.running) return;
    if (launchingAgents.has(currentActiveTab)) return;
    if (autoLaunchedRef.current.has(currentActiveTab)) return;

    autoLaunchedRef.current.add(currentActiveTab);
    handleLaunch(currentActiveTab);
  }, [currentActiveTab, selectedInstance, instances, agents, launchingAgents]);

  const closeAgentTab = useCallback((agentId: string) => {
    if (!selectedInstance) return;
    autoLaunchedRef.current.delete(agentId);
    setOpenAgentTabs((prev) => {
      const m = new Map(prev);
      const tabs = (m.get(selectedInstance) ?? []).filter((id) => id !== agentId);
      m.set(selectedInstance, tabs);
      return m;
    });
    // Switch to another tab if closing the active one
    setActiveAgentTab((prev) => {
      if (prev.get(selectedInstance) !== agentId) return prev;
      const tabs = (openAgentTabs.get(selectedInstance) ?? []).filter((id) => id !== agentId);
      const next = new Map(prev);
      if (tabs.length > 0) {
        next.set(selectedInstance, tabs[0]);
      } else {
        next.delete(selectedInstance);
      }
      return next;
    });
  }, [selectedInstance, openAgentTabs]);

  const handleNewTerminal = useCallback(async () => {
    if (!selectedInstance) return;
    setCreatingTerminal(true);
    try {
      const res = await terminalClient.createTerminal({ instanceName: selectedInstance, name: "" });
      if (res.terminal) setActiveTerminal(res.terminal.id);
    } catch (e: any) {
      console.error("createTerminal failed:", e);
    } finally {
      setCreatingTerminal(false);
    }
  }, [selectedInstance]);

  const handleCloseTerminal = useCallback(async (id: string) => {
    try {
      await terminalClient.closeTerminal({ id });
      if (activeTerminal === id) setActiveTerminal(null);
    } catch { /* ignore */ }
  }, [activeTerminal]);

  const handleLaunch = useCallback(async (agentId: string) => {
    if (!selectedInstance) return;
    setLaunchingAgents((prev) => new Set(prev).add(agentId));
    try {
      await agentClient.launchAgent({ instanceName: selectedInstance, sessionId: agentId, prompt: "" });
    } catch (e: any) {
      console.error("launchAgent failed:", e);
    } finally {
      setLaunchingAgents((prev) => {
        const next = new Set(prev);
        next.delete(agentId);
        return next;
      });
    }
  }, [selectedInstance]);

  const handleStop = useCallback(async (agentId: string) => {
    if (!selectedInstance) return;
    try {
      await agentClient.stopAgent({ instanceName: selectedInstance, sessionId: agentId });
    } catch (e: any) {
      console.error("stopAgent failed:", e);
    }
  }, [selectedInstance]);

  // Instance actions (for sidebar)
  const handleStartInstance = useCallback(async (name: string) => {
    try {
      await containerClient.startContainer({ name, removeExisting: true });
    } catch (e: any) {
      console.error("startContainer failed:", e);
    }
  }, []);

  const handleStopInstance = useCallback(async (name: string) => {
    try {
      await containerClient.stopContainer({ name });
    } catch (e: any) {
      console.error("stopContainer failed:", e);
    }
  }, []);

  const handleDeleteInstance = useCallback(async (name: string) => {
    if (!confirm(`Delete instance "${name}"?`)) return;
    try {
      await containerClient.deleteContainer({ name });
      if (selectedInstance === name) setSelectedInstance(null);
    } catch (e: any) {
      console.error("deleteContainer failed:", e);
    }
  }, [selectedInstance]);

  // --- Resize handlers ---
  const handleSidebarResize = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    const startX = e.clientX;
    const startWidth = sidebarWidth;

    const onMouseMove = (ev: MouseEvent) => {
      const delta = ev.clientX - startX;
      setSidebarWidth(Math.max(MIN_SIDEBAR, Math.min(MAX_SIDEBAR, startWidth + delta)));
    };
    const onMouseUp = () => {
      document.removeEventListener("mousemove", onMouseMove);
      document.removeEventListener("mouseup", onMouseUp);
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
    };
    document.addEventListener("mousemove", onMouseMove);
    document.addEventListener("mouseup", onMouseUp);
    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
  }, [sidebarWidth]);

  const handleBottomResize = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    const startY = e.clientY;
    const startHeight = bottomHeight;
    const maxBottom = mainRef.current ? mainRef.current.clientHeight * MAX_BOTTOM_RATIO : 600;

    const onMouseMove = (ev: MouseEvent) => {
      const delta = startY - ev.clientY;
      setBottomHeight(Math.max(MIN_BOTTOM, Math.min(maxBottom, startHeight + delta)));
    };
    const onMouseUp = () => {
      document.removeEventListener("mousemove", onMouseMove);
      document.removeEventListener("mouseup", onMouseUp);
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
    };
    document.addEventListener("mousemove", onMouseMove);
    document.addEventListener("mouseup", onMouseUp);
    document.body.style.cursor = "row-resize";
    document.body.style.userSelect = "none";
  }, [bottomHeight]);

  return (
    <Box style={{ height: "100vh", display: "flex", flexDirection: "column", overflow: "hidden" }}>
      {/* Main area */}
      <Box style={{ flex: 1, display: "flex", minHeight: 0, overflow: "hidden" }}>
        {/* Sidebar */}
        <Box bg="dark.7" style={{ width: sidebarWidth, flexShrink: 0, overflow: "hidden" }}>
          <Sidebar
            instances={instances}
            agents={agentsMap}
            selectedInstance={selectedInstance}
            onSelectInstance={setSelectedInstance}
            onSelectAgent={openAgentTab}
            onNewInstance={() => setCreateInstanceOpen(true)}
            onNewAgent={(name) => setCreateAgentFor(name)}
            onStartInstance={handleStartInstance}
            onStopInstance={handleStopInstance}
            onDeleteInstance={handleDeleteInstance}
          />
        </Box>

        {/* Sidebar resize handle */}
        <Box
          onMouseDown={handleSidebarResize}
          style={{
            width: 4,
            cursor: "col-resize",
            flexShrink: 0,
            background: "var(--mantine-color-dark-4)",
            transition: "background 0.15s",
          }}
          onMouseEnter={(e) => (e.currentTarget.style.background = "var(--mantine-color-blue-7)")}
          onMouseLeave={(e) => (e.currentTarget.style.background = "var(--mantine-color-dark-4)")}
        />

        {/* Main content */}
        <Box ref={mainRef} style={{ flex: 1, display: "flex", flexDirection: "column", minWidth: 0, overflow: "hidden" }}>
          {/* Agent tabs area */}
          <Box style={{ flex: 1, minHeight: 0, overflow: "hidden", position: "relative" }}>
            <AgentTabs
              agents={agents}
              openTabs={currentOpenTabs}
              activeTab={currentActiveTab}
              launchingAgents={launchingAgents}
              onSelectTab={(id) =>
                selectedInstance &&
                setActiveAgentTab((prev) => new Map(prev).set(selectedInstance, id))
              }
              onCloseTab={closeAgentTab}
              onLaunch={handleLaunch}
              onStop={handleStop}
            />
          </Box>

          {/* Bottom resize handle */}
          <Box
            onMouseDown={handleBottomResize}
            style={{
              height: 4,
              cursor: "row-resize",
              flexShrink: 0,
              background: "var(--mantine-color-dark-4)",
              transition: "background 0.15s",
            }}
            onMouseEnter={(e) => (e.currentTarget.style.background = "var(--mantine-color-blue-7)")}
            onMouseLeave={(e) => (e.currentTarget.style.background = "var(--mantine-color-dark-4)")}
          />

          {/* Bottom panel */}
          <Box style={{ height: bottomHeight, flexShrink: 0, overflow: "hidden" }}>
            <BottomPanel
              terminals={terminals}
              activeTerminal={activeTerminal}
              creatingTerminal={creatingTerminal}
              onSelectTerminal={setActiveTerminal}
              onNewTerminal={handleNewTerminal}
              onCloseTerminal={handleCloseTerminal}
              logs={logs}
              logsActive={logsActive}
              activeBottomTab={activeBottomTab}
              onSelectBottomTab={setActiveBottomTab}
            />
          </Box>
        </Box>
      </Box>

      {/* Status bar */}
      <StatusBar
        instances={instances}
        liveAgentCount={liveAgentCount}
        selectedInstance={selectedInstance}
        selectedAgentName={activeAgentName}
      />

      {/* Modals */}
      <CreateInstanceModal opened={createInstanceOpen} onClose={() => setCreateInstanceOpen(false)} />
      {createAgentFor && (
        <CreateAgentModal opened instanceName={createAgentFor} onClose={() => setCreateAgentFor(null)} />
      )}
    </Box>
  );
}
