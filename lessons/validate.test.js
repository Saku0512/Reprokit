const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const assert = require("node:assert/strict");
const { validateLesson } = require("./validate");

const fixtures = path.join(__dirname, "fixtures");
const read = (name) => JSON.parse(fs.readFileSync(path.join(fixtures, name), "utf8"));

test("valid lesson uses a descriptive title and versioned logical Lab refs", () => {
  assert.deepEqual(validateLesson(read("valid.json")), []);
});

const invalidCases = {
  "cve-as-title.json": "/metadata/title",
  "duplicate-lab-ref.json": "duplicate lab ref",
  "missing-license.json": "required property 'license'",
  "missing-source.json": "/spec/evidence/references",
  "unknown-step-lab.json": "unknown step labRef",
  "unsafe-lab-id.json": "/spec/labs/0/labId",
  "wrong-fixed-lab.json": "fixedLabRef must name a compare-fix lab",
};
assert.deepEqual(
  fs.readdirSync(path.join(fixtures, "invalid")).sort(),
  Object.keys(invalidCases).sort(),
);
for (const [file, expected] of Object.entries(invalidCases)) {
  test(`rejects ${file}`, () => {
    const errors = validateLesson(read(path.join("invalid", file)));
    assert.ok(
      errors.some((error) => error.includes(expected)),
      `${file}: ${errors.join("; ")}`,
    );
  });
}

test("CVE identifier is optional evidence, not the Lesson title or ID", () => {
  const lesson = read("valid.json");
  delete lesson.spec.evidence.cves;
  assert.deepEqual(validateLesson(lesson), []);
});

test("separate Lessons may reuse the same Lab ID", () => {
  const lesson = read("valid.json");
  lesson.spec.labs[1].labId = lesson.spec.labs[0].labId;
  assert.deepEqual(validateLesson(lesson), []);
});

test("rejects runtime source and executable fields in a Lesson", () => {
  const lesson = read("valid.json");
  lesson.spec.labs[0].gitUrl = "https://example.com/repository.git";
  lesson.spec.steps[0].command = "echo marker";
  const errors = validateLesson(lesson);
  assert.ok(errors.some((error) => error.includes("/spec/labs/0")));
  assert.ok(errors.some((error) => error.includes("/spec/steps/0")));
});
