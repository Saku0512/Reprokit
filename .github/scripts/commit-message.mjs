const subjectPattern =
  /^(feat|fix|docs|test|ci|refactor|chore)\(([a-z0-9]+(?:-[a-z0-9]+)*)\): (\S(?:.*\S)?)$/u;

export function isValidSubject(subject) {
  return typeof subject === "string" && !/[\r\n]/u.test(subject) && subjectPattern.test(subject);
}
