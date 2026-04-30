import * as vscode from "vscode";
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
  TransportKind,
} from "vscode-languageclient/node";

let client: LanguageClient | undefined;

export async function activate(context: vscode.ExtensionContext): Promise<void> {
  const command = resolveServerCommand();

  const serverOptions: ServerOptions = {
    command,
    args: [],
    transport: TransportKind.stdio,
  };

  const clientOptions: LanguageClientOptions = {
    documentSelector: [{ scheme: "file", language: "cadence" }],
    outputChannelName: "cadencefmt",
    // The trace.server setting is read by the language client itself via the
    // configuration section name passed below.
    synchronize: {
      configurationSection: "cadencefmt",
    },
  };

  client = new LanguageClient(
    "cadencefmt",
    "cadencefmt",
    serverOptions,
    clientOptions,
  );

  context.subscriptions.push(
    vscode.commands.registerCommand("cadencefmt.formatDocument", async () => {
      const editor = vscode.window.activeTextEditor;
      if (!editor) {
        vscode.window.showInformationMessage(
          "cadencefmt: no active editor.",
        );
        return;
      }
      if (editor.document.languageId !== "cadence") {
        vscode.window.showInformationMessage(
          "cadencefmt: active document is not Cadence.",
        );
        return;
      }
      await vscode.commands.executeCommand("editor.action.formatDocument");
    }),
  );

  try {
    await client.start();
  } catch (err) {
    const message =
      err instanceof Error ? err.message : String(err);
    const action = await vscode.window.showErrorMessage(
      `cadencefmt: failed to start language server (${message}). ` +
        `Verify cadencefmt-lsp is installed and on PATH, or set cadencefmt.path.`,
      "Open Settings",
    );
    if (action === "Open Settings") {
      await vscode.commands.executeCommand(
        "workbench.action.openSettings",
        "cadencefmt.path",
      );
    }
  }

  // Fire-and-forget; never block activation on the network.
  void checkForUpdates(context);
}

export function deactivate(): Thenable<void> | undefined {
  return client?.stop();
}

function resolveServerCommand(): string {
  const config = vscode.workspace.getConfiguration("cadencefmt");
  const command = config.get<string>("path");
  if (typeof command === "string" && command.trim().length > 0) {
    return command;
  }
  return "cadencefmt-lsp";
}

const RELEASES_API =
  "https://api.github.com/repos/janezpodhostnik/cadencefmt/releases/latest";
const UPDATE_CHECK_INTERVAL_MS = 24 * 60 * 60 * 1000;
const LAST_CHECK_KEY = "cadencefmt.lastUpdateCheck";

async function checkForUpdates(
  context: vscode.ExtensionContext,
): Promise<void> {
  const config = vscode.workspace.getConfiguration("cadencefmt");
  if (!config.get<boolean>("checkForUpdates", true)) {
    return;
  }

  const lastCheck = context.globalState.get<number>(LAST_CHECK_KEY, 0);
  if (Date.now() - lastCheck < UPDATE_CHECK_INTERVAL_MS) {
    return;
  }
  await context.globalState.update(LAST_CHECK_KEY, Date.now());

  let response: Response;
  try {
    response = await fetch(RELEASES_API, {
      headers: { Accept: "application/vnd.github+json" },
    });
  } catch {
    // Offline or DNS failure — silent.
    return;
  }
  if (!response.ok) {
    // Rate-limited or other transient — silent.
    return;
  }

  const release = (await response.json()) as {
    tag_name?: string;
    html_url?: string;
  };
  const latest = release.tag_name?.replace(/^v/, "");
  if (!latest) {
    return;
  }

  const current = context.extension.packageJSON.version as string;
  if (compareSemver(latest, current) <= 0) {
    return;
  }

  const action = await vscode.window.showInformationMessage(
    `cadencefmt ${latest} is available (you have ${current}).`,
    "Open Release",
    "Don't show again",
  );
  if (action === "Open Release" && release.html_url) {
    await vscode.env.openExternal(vscode.Uri.parse(release.html_url));
  } else if (action === "Don't show again") {
    await config.update(
      "checkForUpdates",
      false,
      vscode.ConfigurationTarget.Global,
    );
  }
}

// compareSemver compares two dotted-numeric versions. Returns negative if a < b,
// zero if a == b, positive if a > b. Pre-release suffixes are ignored.
function compareSemver(a: string, b: string): number {
  const pa = a.split("-")[0].split(".").map((n) => Number.parseInt(n, 10) || 0);
  const pb = b.split("-")[0].split(".").map((n) => Number.parseInt(n, 10) || 0);
  for (let i = 0; i < Math.max(pa.length, pb.length); i++) {
    const da = pa[i] ?? 0;
    const db = pb[i] ?? 0;
    if (da !== db) {
      return da - db;
    }
  }
  return 0;
}
