import { spawn } from "node:child_process";
import { once } from "node:events";
import { mkdtemp, rm } from "node:fs/promises";
import { homedir, tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { createWebServer, runnerConfig } from "./server.mjs";

const repositoryRoot = fileURLToPath(new URL("../", import.meta.url));

export function stateDirectory(env = process.env) {
  return resolve(
    env.REPROKIT_STATE_DIR ||
      join(env.XDG_STATE_HOME || join(homedir(), ".local", "state"), "reprokit"),
  );
}

function childEnvironment(state) {
  const env = { ...process.env };
  for (const key of Object.keys(env)) if (key.startsWith("GIT_")) delete env[key];
  return {
    ...env,
    REPROKIT_REPOSITORY_ROOT: repositoryRoot,
    REPROKIT_STATE_DIR: state,
    REPROKIT_BOOTSTRAP_FD: "3",
  };
}

function bootstrap(child) {
  return new Promise((resolve, reject) => {
    let data = "";
    const timeout = setTimeout(
      () => finish(new Error("Runnerの初期化がタイムアウトしました。")),
      30000,
    );
    const onExit = () => finish(new Error("Runnerが起動に失敗しました。"));
    const onError = () => finish(new Error("Runnerを起動できませんでした。"));
    function finish(error, value) {
      clearTimeout(timeout);
      child.off("exit", onExit);
      child.off("error", onError);
      child.stdio[3].removeAllListeners("data");
      if (error) reject(error);
      else resolve(value);
    }
    child.once("exit", onExit);
    child.once("error", onError);
    child.stdio[3].on("data", (chunk) => {
      data += chunk.toString();
      if (data.length > 4096) {
        finish(new Error("Runnerの初期化応答が不正です。"));
        return;
      }
      if (!data.includes("\n")) return;
      try {
        const value = JSON.parse(data);
        const config = runnerConfig(value.origin, value.token);
        if (!config) throw new Error("missing Runner configuration");
        finish(null, config);
      } catch {
        finish(new Error("Runnerの初期化応答が不正です。"));
      }
    });
  });
}

// ビルドしたGo Runnerを子プロセスとして所有し、認証情報は私有パイプだけで受け取る。
export async function launch({
  port = 4173,
  dist = true,
  state = stateDirectory(),
  onRunnerExit = () => {},
} = {}) {
  const buildDirectory = await mkdtemp(join(tmpdir(), "reprokit-runner-"));
  const binary = join(buildDirectory, "reprokit");
  let child,
    server,
    closing,
    closed = false;
  const env = childEnvironment(state);
  function close() {
    if (closing) return closing;
    closed = true;
    closing = (async () => {
      if (server?.listening) {
        server.closeAllConnections();
        await new Promise((resolve) => server.close(resolve));
      }
      if (child && child.exitCode === null && child.signalCode === null) {
        const exited = once(child, "exit");
        child.kill("SIGTERM");
        const kill = setTimeout(() => child.kill("SIGKILL"), 6000);
        try {
          await exited;
        } finally {
          clearTimeout(kill);
        }
      }
      await rm(buildDirectory, { recursive: true, force: true });
    })();
    return closing;
  }
  try {
    const build = spawn("go", ["build", "-o", binary, "./cmd/reprokit"], {
      cwd: join(repositoryRoot, "backend"),
      env,
      stdio: ["ignore", "inherit", "inherit"],
    });
    const [code] = await once(build, "exit");
    if (code !== 0) throw new Error("Runnerのビルドに失敗しました。");
    child = spawn(binary, [], { env, stdio: ["ignore", "inherit", "inherit", "pipe"] });
    const runner = await bootstrap(child);
    server = createWebServer({ runner, dist });
    child.once("exit", () => {
      if (!closed) {
        onRunnerExit();
        void close();
      }
    });
    await new Promise((resolve, reject) => {
      server.once("error", reject);
      server.listen(port, "127.0.0.1", resolve);
    });
    return { origin: `http://127.0.0.1:${server.address().port}`, server, child, close };
  } catch (error) {
    await close();
    throw error;
  }
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const port = Number(process.env.REPROKIT_WEB_PORT ?? 4173);
  if (!Number.isInteger(port) || port < 1 || port > 65535) throw new Error("Invalid Web port");
  try {
    console.log("Reprokit: ローカルRunnerを起動しています…");
    const app = await launch({
      port,
      dist: !process.argv.includes("--dev"),
      onRunnerExit: () => {
        console.error("Reprokit: Runnerが終了したためWebを停止します。");
        process.exitCode = 1;
      },
    });
    console.log(`Reprokit Web: ${app.origin}`);
    for (const signal of ["SIGINT", "SIGTERM"])
      process.once(signal, () => {
        void app.close();
      });
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
