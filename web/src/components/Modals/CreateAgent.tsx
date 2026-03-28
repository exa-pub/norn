import { Button, Modal, TextInput } from "@mantine/core";
import { useState } from "react";
import { agentClient } from "../../client";

interface Props {
  opened: boolean;
  instanceName: string;
  onClose: () => void;
}

export function CreateAgentModal({ opened, instanceName, onClose }: Props) {
  const [name, setName] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const submit = async () => {
    setLoading(true);
    setError("");
    try {
      await agentClient.createAgentSession({ instanceName, name });
      setName("");
      onClose();
    } catch (e: any) {
      setError(e.message || "Failed to create");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal opened={opened} onClose={onClose} title={`New Agent — ${instanceName}`}>
      <TextInput
        label="Session name"
        placeholder="fix-layout-bug"
        value={name}
        onChange={(e) => setName(e.currentTarget.value)}
        error={error}
        onKeyDown={(e) => e.key === "Enter" && submit()}
      />
      <Button mt="md" onClick={submit} loading={loading} fullWidth>
        Create
      </Button>
    </Modal>
  );
}
