import { Button, Modal, TextInput } from "@mantine/core";
import { useState } from "react";
import { containerClient } from "../../client";

interface Props {
  opened: boolean;
  onClose: () => void;
}

export function CreateInstanceModal({ opened, onClose }: Props) {
  const [name, setName] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const submit = async () => {
    setLoading(true);
    setError("");
    try {
      await containerClient.create({ name });
      setName("");
      onClose();
    } catch (e: any) {
      setError(e.message || "Failed to create");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal opened={opened} onClose={onClose} title="New Instance">
      <TextInput
        label="Name"
        placeholder="my-instance"
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
