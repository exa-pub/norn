import { Box, Button, Group, Modal, Text, TextInput } from "@mantine/core";
import { notifications } from "@mantine/notifications";
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
import { useDocumentTitle } from "./hooks/useDocumentTitle";
import { useOptimistic } from "./hooks/useOptimistic";
import { agentClient, containerClient, terminalClient } from "./client";
import { ContainerStatus } from "./gen/norn/containers/v1/containers_pb";
import type { AgentSession } from "./gen/norn/agents/v1/agents_pb";
import type { StreamLogsResponse } from "./gen/norn/containers/v1/containers_pb";

const MIN_SIDEBAR = 180;
const MAX_SIDEBAR = 500;
const MIN_BOTTOM = 80;
const MAX_BOTTOM_RATIO = 0.7;

const getInstanceId = (i: { name: string }) => i.name;
const getAgentId = (a: { id: string }) => a.id;
const getTerminalId = (t: { id: string }) => t.id;

export function App() {
  const { instances: polledInstances, loading: instancesLoading } = useInstances();
  const [selectedInstance, setSelectedInstance] = useState<string | null>(null);

  // Optimistic wrappers
  const instOpt = useOptimistic(polledInstances, getInstanceId);
  const instances = instOpt.items;

  const polledAgents = useAgents(selectedInstance);
  const agentOpt = useOptimistic(polledAgents, getAgentId);
  const agents = agentOpt.items;

  const polledTerminals = useTerminals(selectedInstance);
  const termOpt = useOptimistic(polledTerminals, getTerminalId);
  const terminals = termOpt.items;

  // Per-instance open tabs
  const [openAgentTabs, setOpenAgentTabs] = useState<Map<string, string[]>>(new Map());
  const [activeAgentTab, setActiveAgentTab] = useState<Map<string, string>>(new Map());
  const [activeTerminal, setActiveTerminal] = useState<string | null>(null);

  // Loading states
  const [creatingTerminal, setCreatingTerminal] = useState(false);

  // Modals
  const [createInstanceOpen, setCreateInstanceOpen] = useState(false);
  const [createAgentFor, setCreateAgentFor] = useState<string | null>(null);
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);
  const [renameAgent, setRenameAgent] = useState<{ instanceName: string; agentId: string } | null>(null);
  const [renameValue, setRenameValue] = useState("");
  const [renameTerminal, setRenameTerminal] = useState<{ id: string } | null>(null);
  const [renameTerminalValue, setRenameTerminalValue] = useState("");

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

  // Detect agent completion (running → stopped) and notify
  const prevAgentsRef = useRef<Map<string, boolean>>(new Map());
  useEffect(() => {
    const prev = prevAgentsRef.current;
    for (const agent of agents) {
      const wasRunning = prev.get(agent.id);
      if (wasRunning && !agent.running && !agent._pending) {
        notifications.show({
          title: "Agent finished",
          message: agent.name || agent.id.slice(0, 8),
          color: "blue",
        });
      }
    }
    prevAgentsRef.current = new Map(agents.map((a) => [a.id, a.running]));
  }, [agents]);

  // Resolve active agent name for status bar
  const activeAgentName = useMemo(() => {
    if (!currentActiveTab) return null;
    const agent = agents.find((a) => a.id === currentActiveTab);
    return agent?.name || currentActiveTab.slice(0, 8);
  }, [currentActiveTab, agents]);

  useDocumentTitle(selectedInstance, activeAgentName, liveAgentCount);

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

  // Auto-launch effect
  useEffect(() => {
    if (!selectedInstance || !currentActiveTab) return;
    const inst = instances.find((i) => i.name === selectedInstance);
    if (!inst || inst.status !== ContainerStatus.RUNNING) return;

    const agent = agents.find((a) => a.id === currentActiveTab);
    if (!agent || agent.running || agent._pending) return;
    if (autoLaunchedRef.current.has(currentActiveTab)) return;

    autoLaunchedRef.current.add(currentActiveTab);
    handleLaunch(currentActiveTab);
  }, [currentActiveTab, selectedInstance, instances, agents]);

  const closeAgentTab = useCallback((agentId: string) => {
    if (!selectedInstance) return;
    autoLaunchedRef.current.delete(agentId);
    setOpenAgentTabs((prev) => {
      const m = new Map(prev);
      const tabs = (m.get(selectedInstance) ?? []).filter((id) => id !== agentId);
      m.set(selectedInstance, tabs);
      return m;
    });
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

  // ─── Terminal actions ───

  const handleNewTerminal = useCallback(async () => {
    if (!selectedInstance) return;
    setCreatingTerminal(true);
    try {
      const res = await terminalClient.createTerminal({ instanceName: selectedInstance, name: "" });
      if (res.terminal) {
        termOpt.addOptimistic(res.terminal);
        setActiveTerminal(res.terminal.id);
      }
    } catch (e: any) {
      notifications.show({ title: "Terminal error", message: e?.message ?? "Failed to create terminal", color: "red" });
    } finally {
      setCreatingTerminal(false);
    }
  }, [selectedInstance, termOpt]);

  const handleCloseTerminal = useCallback(async (id: string) => {
    termOpt.hide(id);
    if (activeTerminal === id) setActiveTerminal(null);
    try {
      await terminalClient.closeTerminal({ id });
    } catch {
      termOpt.clear(id); // unhide on error
    }
  }, [activeTerminal, termOpt]);

  const handleRenameTerminalOpen = useCallback((id: string) => {
    const t = terminals.find((t) => t.id === id);
    setRenameTerminalValue(t?.name || "");
    setRenameTerminal({ id });
  }, [terminals]);

  const confirmRenameTerminal = useCallback(async () => {
    if (!renameTerminal) return;
    const { id } = renameTerminal;
    const newName = renameTerminalValue;
    setRenameTerminal(null);
    termOpt.patch(id, { name: newName } as Partial<typeof terminals[0]>);
    try {
      await terminalClient.renameTerminal({ id, name: newName });
    } catch (e: any) {
      termOpt.clear(id);
      notifications.show({ title: "Terminal error", message: e?.message ?? "Failed to rename terminal", color: "red" });
    }
  }, [renameTerminal, renameTerminalValue, termOpt]);

  // ─── Agent actions ───

  const handleLaunch = useCallback(async (agentId: string) => {
    if (!selectedInstance) return;
    agentOpt.patch(agentId, { running: true } as Partial<AgentSession>);
    agentOpt.setPending(agentId, true);
    try {
      await agentClient.launchAgent({ instanceName: selectedInstance, sessionId: agentId, prompt: "" });
    } catch (e: any) {
      agentOpt.clear(agentId);
      notifications.show({ title: "Agent error", message: e?.message ?? "Failed to launch agent", color: "red" });
    } finally {
      agentOpt.setPending(agentId, false);
    }
  }, [selectedInstance, agentOpt]);

  const handleStopAgentSidebar = useCallback(async (instanceName: string, agentId: string) => {
    agentOpt.patch(agentId, { running: false } as Partial<AgentSession>);
    try {
      await agentClient.stopAgent({ instanceName, sessionId: agentId });
    } catch (e: any) {
      agentOpt.clear(agentId);
      notifications.show({ title: "Agent error", message: e?.message ?? "Failed to stop agent", color: "red" });
    }
  }, [agentOpt]);

  const handleLaunchAgentSidebar = useCallback(async (instanceName: string, agentId: string) => {
    agentOpt.patch(agentId, { running: true } as Partial<AgentSession>);
    agentOpt.setPending(agentId, true);
    try {
      await agentClient.launchAgent({ instanceName, sessionId: agentId, prompt: "" });
    } catch (e: any) {
      agentOpt.clear(agentId);
      notifications.show({ title: "Agent error", message: e?.message ?? "Failed to launch agent", color: "red" });
    } finally {
      agentOpt.setPending(agentId, false);
    }
  }, [agentOpt]);

  const handleRenameAgent = useCallback((instanceName: string, agentId: string) => {
    const agent = agents.find((a) => a.id === agentId);
    setRenameValue(agent?.name || "");
    setRenameAgent({ instanceName, agentId });
  }, [agents]);

  const confirmRename = useCallback(async () => {
    if (!renameAgent) return;
    const { instanceName, agentId } = renameAgent;
    const newName = renameValue;
    setRenameAgent(null);
    agentOpt.patch(agentId, { name: newName } as Partial<AgentSession>);
    try {
      await agentClient.updateAgentSessionName({ instanceName, sessionId: agentId, name: newName });
    } catch (e: any) {
      agentOpt.clear(agentId);
      notifications.show({ title: "Agent error", message: e?.message ?? "Failed to rename agent", color: "red" });
    }
  }, [renameAgent, renameValue, agentOpt]);

  const handleDeleteAgent = useCallback(async (instanceName: string, agentId: string) => {
    agentOpt.hide(agentId);
    try {
      await agentClient.deleteAgentSession({ instanceName, sessionId: agentId });
    } catch (e: any) {
      agentOpt.clear(agentId);
      notifications.show({ title: "Agent error", message: e?.message ?? "Failed to delete agent", color: "red" });
    }
  }, [agentOpt]);

  // ─── Instance actions ───

  const handleStartInstance = useCallback(async (name: string) => {
    instOpt.patch(name, { status: ContainerStatus.STARTING } as any);
    try {
      await containerClient.startContainer({ name, removeExisting: true });
    } catch (e: any) {
      instOpt.clear(name);
      notifications.show({ title: "Instance error", message: e?.message ?? "Failed to start instance", color: "red" });
    }
  }, [instOpt]);

  const handleStopInstance = useCallback(async (name: string) => {
    instOpt.patch(name, { status: ContainerStatus.STOPPED } as any);
    try {
      await containerClient.stopContainer({ name });
    } catch (e: any) {
      instOpt.clear(name);
      notifications.show({ title: "Instance error", message: e?.message ?? "Failed to stop instance", color: "red" });
    }
  }, [instOpt]);

  const handleDeleteInstance = useCallback(async (name: string) => {
    setDeleteConfirm(name);
  }, []);

  const confirmDelete = useCallback(async () => {
    if (!deleteConfirm) return;
    const name = deleteConfirm;
    setDeleteConfirm(null);
    instOpt.hide(name);
    if (selectedInstance === name) setSelectedInstance(null);
    try {
      await containerClient.deleteContainer({ name });
    } catch (e: any) {
      instOpt.clear(name);
      notifications.show({ title: "Instance error", message: e?.message ?? "Failed to delete instance", color: "red" });
    }
  }, [deleteConfirm, selectedInstance, instOpt]);

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
            loading={instancesLoading}
            onSelectInstance={setSelectedInstance}
            onSelectAgent={openAgentTab}
            onNewInstance={() => setCreateInstanceOpen(true)}
            onNewAgent={(name) => setCreateAgentFor(name)}
            onStartInstance={handleStartInstance}
            onStopInstance={handleStopInstance}
            onDeleteInstance={handleDeleteInstance}
            onStopAgent={handleStopAgentSidebar}
            onLaunchAgent={handleLaunchAgentSidebar}
            onDeleteAgent={handleDeleteAgent}
            onRenameAgent={handleRenameAgent}
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
              launchingAgents={new Set(agents.filter((a) => a._pending).map((a) => a.id))}
              onSelectTab={(id) =>
                selectedInstance &&
                setActiveAgentTab((prev) => new Map(prev).set(selectedInstance, id))
              }
              onCloseTab={closeAgentTab}
              onLaunch={handleLaunch}
              onReorderTabs={(tabs) => {
                if (!selectedInstance) return;
                setOpenAgentTabs((prev) => new Map(prev).set(selectedInstance, tabs));
              }}
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
              onRenameTerminal={handleRenameTerminalOpen}
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

      <Modal opened={!!deleteConfirm} onClose={() => setDeleteConfirm(null)} title="Delete Instance" size="sm">
        <Text size="sm">Are you sure you want to delete <b>{deleteConfirm}</b>? This action cannot be undone.</Text>
        <Group justify="flex-end" mt="md">
          <Button variant="default" onClick={() => setDeleteConfirm(null)}>Cancel</Button>
          <Button color="red" onClick={confirmDelete}>Delete</Button>
        </Group>
      </Modal>

      <Modal opened={!!renameAgent} onClose={() => setRenameAgent(null)} title="Rename Agent" size="sm">
        <TextInput
          label="Name"
          value={renameValue}
          onChange={(e) => setRenameValue(e.currentTarget.value)}
          onKeyDown={(e) => e.key === "Enter" && confirmRename()}
          data-autofocus
        />
        <Group justify="flex-end" mt="md">
          <Button variant="default" onClick={() => setRenameAgent(null)}>Cancel</Button>
          <Button onClick={confirmRename}>Rename</Button>
        </Group>
      </Modal>

      <Modal opened={!!renameTerminal} onClose={() => setRenameTerminal(null)} title="Rename Terminal" size="sm">
        <TextInput
          label="Name"
          value={renameTerminalValue}
          onChange={(e) => setRenameTerminalValue(e.currentTarget.value)}
          onKeyDown={(e) => e.key === "Enter" && confirmRenameTerminal()}
          data-autofocus
        />
        <Group justify="flex-end" mt="md">
          <Button variant="default" onClick={() => setRenameTerminal(null)}>Cancel</Button>
          <Button onClick={confirmRenameTerminal}>Rename</Button>
        </Group>
      </Modal>
    </Box>
  );
}
