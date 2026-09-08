// IPC types — hand-mirrored from src-tauri/src/ipc_types.rs.
// In phase 2 we will use `ts-rs` to auto-generate this file from Rust.

export interface ProjectMeta {
  id: string;
  name: string;
  path: string;
  vertexCount: number;
  taskCount: number;
  conflictCount: number;
  lastOpenedAt: string | null;
  initialized: boolean;
}

export interface Greeting {
  message: string;
  backend: string;
  timestamp: string;
}
