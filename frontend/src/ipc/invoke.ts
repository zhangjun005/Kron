import { invoke } from '@tauri-apps/api/core';
import type { Greeting, ProjectMeta } from '../types/ipc';

/**
 * Type-safe wrapper around Tauri `invoke('kron_*', args)`.
 * Mirrors `dev-docs/design/05-GUI设计.md` § 3.2 IPC contract.
 */
export const ipc = {
  kronProjectList(): Promise<ProjectMeta[]> {
    return invoke<ProjectMeta[]>('kron_project_list');
  },

  kronGreet(name: string): Promise<Greeting> {
    return invoke<Greeting>('kron_greet', { name });
  },
};
