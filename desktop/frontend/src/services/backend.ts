import * as FastgitAPI from "../../bindings/fastgitdesktop/fastgitservice";

import type { ActionRunRequest, CommandResult, DesktopModule, GitHubAuthStatus } from "../app/types";

export class BackendService {
  getRepoRoot(): Promise<string> {
    return FastgitAPI.GetRepoRoot();
  }

  setRepoRoot(path: string): Promise<void> {
    return FastgitAPI.SetRepoRoot(path);
  }

  async getModules(): Promise<DesktopModule[]> {
    return (await FastgitAPI.GetModules()) ?? [];
  }

  async getGitHubAuthStatus(): Promise<GitHubAuthStatus> {
    return await FastgitAPI.GetGitHubAuthStatus();
  }

  setGitHubToken(token: string): Promise<void> {
    return FastgitAPI.SetGitHubToken(token);
  }

  runAction(request: ActionRunRequest): Promise<CommandResult> {
    return FastgitAPI.RunAction(request);
  }
}
