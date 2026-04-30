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
