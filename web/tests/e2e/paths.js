import { createHash } from "node:crypto";
import { tmpdir } from "node:os";
import { join } from "node:path";

const workspace = createHash("sha256")
  .update(new URL("../../", import.meta.url).pathname)
  .digest("hex")
  .slice(0, 12);
// 生成JSONをリポジトリの整形対象へ混ぜず、worktreeごとに出力先を分ける。
export const testOutputDir = join(tmpdir(), `reprokit-web-${workspace}`);
