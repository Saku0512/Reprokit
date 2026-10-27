import { execFileSync } from "node:child_process";
import { isValidSubject } from "./commit-message.mjs";

const { PR_TITLE: title, BASE_SHA: base, HEAD_SHA: head } = process.env;
const shaPattern = /^[0-9a-f]{40}$/;

if (!shaPattern.test(base ?? "") || !shaPattern.test(head ?? "")) {
  console.error("BASE_SHA and HEAD_SHA must be full commit hashes.");
  process.exit(1);
}

let failed = false;
function check(label, subject) {
  if (!isValidSubject(subject)) {
    console.error(`${label}: invalid subject ${JSON.stringify(subject)}`);
    failed = true;
  }
}

check("PR title", title);
const commits = execFileSync("git", ["rev-list", "--no-merges", `${base}..${head}`], {
  encoding: "utf8",
})
  .trim()
  .split("\n")
  .filter(Boolean);

for (const commit of commits) {
  const subject = execFileSync("git", ["show", "-s", "--format=%s", commit], {
    encoding: "utf8",
  }).replace(/\n$/, "");
  check(`Commit ${commit}`, subject);
}

if (failed) {
  console.error("Expected <prefix>(<component>): <message>; see CONTRIBUTING.md.");
  process.exit(1);
}
console.log(`PR title and ${commits.length} non-merge commit(s) are valid.`);
