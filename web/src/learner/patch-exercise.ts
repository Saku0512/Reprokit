const setup = 'command := strings.Split(a.Command, " ")';
const mapping = `envMapping := func(key string) string {
    switch key {
    case "USERNAME":
        return a.Cred.Username
    case "PASSWORD":
        return a.Cred.Password
    default:
        return os.Getenv(key)
    }
}`;
const expansion = `for i, arg := range command {
    if i == 0 {
        continue
    }
    command[i] = os.Expand(arg, envMapping)
}`;
const invocation = "cmd := exec.Command(command[0], command[1:]...)";
const usernameEnv = 'cmd.Env = append(os.Environ(), fmt.Sprintf("USERNAME=%s", a.Cred.Username))';
const passwordEnv = 'cmd.Env = append(cmd.Env, fmt.Sprintf("PASSWORD=%s", a.Cred.Password))';

// 演習の元コードと解説差分を同じ抜粋から生成し、章間の食い違いを防ぐ。
const patchSections = [
  { code: setup, removed: false },
  { code: mapping, removed: true },
  { code: expansion, removed: true },
  { code: invocation, removed: false },
  { code: usernameEnv, removed: false },
  { code: passwordEnv, removed: false },
];

export const starterPatch = patchSections.map(({ code }) => code).join("\n");
export const patchComparisonLines = patchSections.flatMap(({ code, removed }) =>
  code.split("\n").map((line) => ({ code: line, removed })),
);

export const patchHints = [
  "usernameとpasswordが、コマンド引数と環境変数のどちらへ渡されるかを追ってください。",
  "envMappingは資格情報を返します。os.Expandの結果は、そのままexec.Commandへ渡されます。",
  "資格情報を引数へ展開するforループと、使わなくなったenvMappingを削除し、cmd.Envの2行を残してみてください。",
];

export interface PatchCheck {
  title: string;
  passed: boolean;
  detail: string;
}

export interface PatchResult {
  passed: boolean;
  error?: string;
  checks: PatchCheck[];
}

// Goの抜粋を字句へ分け、コメント内のコードと空白を判定対象から除く。
// 入力を実行せず、教材で定義した処理の組み合わせだけを確認する。
function tokens(source: string): string[] {
  const parts =
    source.match(
      /\/\/[^\n]*|\/\*[\s\S]*?\*\/|"(?:\\.|[^"\\])*"|`[^`]*`|[a-zA-Z_][a-zA-Z_0-9]*|\d+|\.\.\.|:=|==|[^\s]/g,
    ) ?? [];
  return parts.filter((part) => !part.startsWith("//") && !part.startsWith("/*"));
}

function normalized(source: string): string {
  return JSON.stringify(tokens(source));
}

export function checkPatch(source: string): PatchResult {
  if (source.length > 12000) {
    return { passed: false, error: "コードは12,000文字以内にしてください。", checks: [] };
  }
  const actual = normalized(source);
  // 既知の抜粋に限定し、対象外の字句や処理を修正成功と判定しない。
  for (const hasMapping of [true, false]) {
    for (const expand of [true, false]) {
      for (const username of [true, false]) {
        for (const password of [true, false]) {
          const candidate = [
            setup,
            ...(hasMapping ? [mapping] : []),
            ...(expand ? [expansion] : []),
            invocation,
            ...(username ? [usernameEnv] : []),
            ...(password ? [passwordEnv] : []),
          ].join("\n");
          if (actual !== normalized(candidate)) continue;
          const checks = [
            {
              title: "envMappingと展開ループの参照を整える",
              passed: hasMapping === expand,
              detail:
                hasMapping === expand
                  ? "envMappingと、それを使うループの参照は対応しています。"
                  : hasMapping
                    ? "ループの削除後はenvMappingが未使用になるため、その定義も削除してください。"
                    : "ループが参照するenvMappingがありません。資格情報を展開する処理全体を見直してください。",
            },
            {
              title: "資格情報をコマンド引数へ展開しない",
              passed: !expand,
              detail: expand
                ? "os.Expandで資格情報が引数へ入ります。環境変数の設定だけでは、この処理は取り消されません。"
                : "設定されたコマンドの引数を、資格情報で置き換える処理がありません。",
            },
            {
              title: "USERNAMEを環境変数で渡す",
              passed: username,
              detail: username
                ? "ユーザー名をフックへデータとして渡す処理が残っています。"
                : "ユーザー名を渡す処理が削除されています。フックが資格情報を受け取れるようにしてください。",
            },
            {
              title: "PASSWORDを環境変数で渡す",
              passed: username && password,
              detail:
                username && password
                  ? "パスワードを、USERNAMEを含む環境変数へ追加しています。"
                  : "USERNAMEとPASSWORDの両方を環境変数で渡す2行を残してください。",
            },
          ];
          return { passed: checks.every((check) => check.passed), checks };
        }
      }
    }
  }
  return {
    passed: false,
    error:
      "このチェックは表示した抜粋のenvMapping、forループ、cmd.Envの2行の削除に対応しています。追加した処理や構文は判定できません。元のコードに戻して、必要な変更を試してください。",
    checks: [],
  };
}
