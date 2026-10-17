const fs = require("node:fs");
const path = require("node:path");
const Ajv2020 = require("ajv/dist/2020");
const addFormats = require("ajv-formats");

const schema = JSON.parse(
  fs.readFileSync(path.join(__dirname, "..", "schemas", "lesson-v1alpha1.schema.json"), "utf8"),
);
const ajv = new Ajv2020({ allErrors: true, strict: true });
addFormats(ajv);
const validateSchema = ajv.compile(schema);

function validateLesson(lesson) {
  if (!validateSchema(lesson)) {
    return validateSchema.errors.map((error) => `${error.instancePath || "/"} ${error.message}`);
  }

  const errors = [];
  const labs = new Map();
  for (const lab of lesson.spec.labs) {
    if (labs.has(lab.ref)) errors.push(`duplicate lab ref: ${lab.ref}`);
    labs.set(lab.ref, lab);
  }
  const stepIds = new Set();
  for (const step of lesson.spec.steps) {
    if (stepIds.has(step.id)) errors.push(`duplicate step id: ${step.id}`);
    stepIds.add(step.id);
    if (step.labRef && !labs.has(step.labRef)) errors.push(`unknown step labRef: ${step.labRef}`);
  }
  if (lesson.spec.comparison.fixedLabRef) {
    const ref = lesson.spec.comparison.fixedLabRef;
    if (!labs.has(ref) || labs.get(ref).purpose !== "compare-fix") {
      errors.push(`fixedLabRef must name a compare-fix lab: ${ref}`);
    }
  }
  const questionIds = new Set();
  for (const question of lesson.spec.assessment) {
    if (questionIds.has(question.id)) errors.push(`duplicate assessment id: ${question.id}`);
    questionIds.add(question.id);
  }
  return errors;
}

module.exports = { validateLesson };

if (require.main === module) {
  const files = process.argv.slice(2);
  if (files.length === 0) {
    console.error("Usage: node lessons/validate.js LESSON.json [...]");
    process.exitCode = 2;
  }
  for (const file of files) {
    try {
      const errors = validateLesson(JSON.parse(fs.readFileSync(file, "utf8")));
      if (errors.length) {
        console.error(`${file}: ${errors.join("; ")}`);
        process.exitCode = 1;
      } else {
        console.log(`${file}: valid`);
      }
    } catch (error) {
      console.error(`${file}: ${error.message}`);
      process.exitCode = 1;
    }
  }
}
