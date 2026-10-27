// バックエンド API のクライアント。

export type Fighter = { id: string; name: string };

export type Account = { id: string; displayName: string; mainFighter: string };

export type Me =
  | { loggedIn: false }
  | { loggedIn: true; registered: false }
  | { loggedIn: true; registered: true; account: Account; termsAccepted: boolean };

export type Instance = {
  name: string;
  sourceCodeUrl: string;
  /** 運営者の問い合わせ先（未設定なら空） */
  contact: string;
  loginProviders: { dev: boolean; discord: boolean };
};

export type MatchmakingStatus = {
  state: "idle" | "waiting" | "matched";
  matchId?: string;
  waitingSeconds?: number;
  allowedDiff?: number;
};

export type Report = "win" | "lose" | "cancel";

/** 1ゲームで使ったファイター。characters はプレイヤー1、プレイヤー2 の順。 */
export type Pick = { game: number; characters: [string, string] };

export type MatchSide = {
  player: { id: string; displayName: string; rating: number };
  fighter: string;
  reported: boolean;
  report?: Report;
  /** 自分が報告したゲームごとのファイター（本人にだけ返る） */
  picks?: Pick[];
};

export type Match = {
  id: string;
  /** 対戦をホストしたインスタンス */
  host: string;
  status: "in_progress" | "confirmed" | "cancelled" | "expired";
  /** モデレーションで無効化された（レート計算から除外された）確定済みの対戦 */
  voided?: boolean;
  mismatched: boolean;
  playedAt: string;
  expiresAt: string;
  winner?: string;
  player1: MatchSide;
  player2: MatchSide;
  /** 確定した対戦で記録したゲームごとのファイター（両者の入力が一致しなければ1戦目だけ） */
  recordedPicks?: Pick[];
  myRole?: "player1" | "player2";
  roomId?: string;
  roomPassword?: string;
};

export type Standing = {
  rank?: number;
  accountId: string;
  displayName?: string;
  rating: number;
  matches: number;
  wins: number;
  losses: number;
};

export type Ranking = { season: string; minMatches: number; entries: Standing[] };

export type Terms = { text: string; version: string; custom: boolean };

/** 通報・処分の区分（docs/domain/match-rules.md の「9.1 禁止事項」） */
export const CATEGORY_LABELS: Record<string, string> = {
  cheating: "改造・チート",
  input_device: "不正な入力機器",
  collusion: "八百長",
  false_report: "虚偽の報告",
  unfair_cancel: "認められない中止",
  unauth_stream: "無断配信",
  exposing: "晒し",
  abuse: "暴言・挑発",
  false_flagging: "虚偽の通報",
  other: "その他",
};

/** 利用者が通報で選べる区分 */
export const REPORT_CATEGORIES = [
  "cheating",
  "input_device",
  "collusion",
  "false_report",
  "unfair_cancel",
  "unauth_stream",
  "exposing",
  "abuse",
  "other",
];

/** 処分の理由として選べる区分 */
export const SANCTION_CATEGORIES = [...REPORT_CATEGORIES.slice(0, -1), "false_flagging", "other"];

export type SanctionKind = "caution" | "yellow" | "red";

export const SANCTION_LABELS: Record<SanctionKind, string> = {
  caution: "注意",
  yellow: "イエローカード",
  red: "レッドカード（利用停止）",
};

/** 対戦記録の無効化の事由（連合で共有される） */
export const VOID_REASON_LABELS: Record<string, string> = {
  collusion: "八百長",
  cheating: "改造・チート・不正な入力機器",
  false_report: "虚偽の報告",
  error: "記録の誤り",
  other: "その他",
};

export type Sanction = {
  id: string;
  kind: SanctionKind;
  reason: string;
  /** 運営者のメモ（管理画面でだけ返る） */
  note?: string;
  createdAt: string;
  endsAt?: string;
  liftedAt?: string;
  /** 利用停止中 */
  active: boolean;
};

export type ModerationStatus = { admin: boolean; suspension?: Sanction; sanctions: Sanction[] };

export type AccountRef = { id: string; displayName: string };

export type ModerationReport = {
  id: string;
  reporter: AccountRef;
  target: AccountRef;
  matchHost?: string;
  matchId?: string;
  /** 対象の対戦を自インスタンスがホストした（無効化できる） */
  hostedHere: boolean;
  category: string;
  body: string;
  status: "open" | "resolved";
  resolution?: string;
  createdAt: string;
  resolvedAt?: string;
};

export type NewReport = { targetId: string; matchHost?: string; matchId?: string; category: string; body: string };

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    credentials: "same-origin",
    headers: {
      "Content-Type": "application/json",
      // CSRF 対策の独自ヘッダー（バックエンドが GET 以外で必須にしている）
      "X-Smash-Hub": "1",
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (res.status === 204) return undefined as T;
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new ApiError(res.status, data.message ?? "エラーが発生しました");
  return data as T;
}

export const api = {
  instance: () => request<Instance>("GET", "/api/instance"),
  fighters: () => request<Fighter[]>("GET", "/api/fighters"),
  me: () => request<Me>("GET", "/api/me"),
  devLogin: (name: string) => request<Me>("POST", "/api/auth/dev/login", { name }),
  logout: () => request<void>("POST", "/api/auth/logout"),
  terms: () => request<Terms>("GET", "/api/terms"),
  register: (displayName: string, mainFighter: string, termsVersion: string) =>
    request<Account>("POST", "/api/register", { displayName, mainFighter, termsVersion }),
  agreeTerms: (termsVersion: string) => request<void>("POST", "/api/me/terms", { termsVersion }),
  updateProfile: (displayName: string, mainFighter: string) =>
    request<Account>("PUT", "/api/me/profile", { displayName, mainFighter }),
  search: (fighter: string, rateRange: number) =>
    request<MatchmakingStatus>("POST", "/api/matchmaking/search", { fighter, rateRange }),
  status: () => request<MatchmakingStatus>("GET", "/api/matchmaking/status"),
  cancelSearch: () => request<void>("DELETE", "/api/matchmaking/search"),
  match: (id: string) => request<Match>("GET", `/api/matches/${encodeURIComponent(id)}`),
  setRoom: (id: string, roomId: string) =>
    request<Match>("PUT", `/api/matches/${encodeURIComponent(id)}/room`, { roomId }),
  report: (id: string, result: Report, picks?: Pick[]) =>
    request<Match>("POST", `/api/matches/${encodeURIComponent(id)}/report`, { result, picks }),
  history: (accountId: string) => request<Match[]>("GET", `/api/accounts/${encodeURIComponent(accountId)}/matches`),
  standing: (accountId: string) => request<Standing>("GET", `/api/accounts/${encodeURIComponent(accountId)}/rating`),
  ranking: () => request<Ranking>("GET", "/api/ranking"),
  moderation: () => request<ModerationStatus>("GET", "/api/me/moderation"),
  sendReport: (r: NewReport) => request<void>("POST", "/api/reports", r),
  admin: {
    reports: (status: "open" | "resolved") => request<ModerationReport[]>("GET", `/api/admin/reports?status=${status}`),
    resolve: (id: string, note: string) =>
      request<void>("POST", `/api/admin/reports/${encodeURIComponent(id)}/resolve`, { note }),
    sanctions: (accountId: string) =>
      request<{ account: AccountRef; sanctions: Sanction[] }>(
        "GET",
        `/api/admin/accounts/${encodeURIComponent(accountId)}/sanctions`,
      ),
    sanction: (s: { accountId: string; kind: SanctionKind; reason: string; note: string; days: number }) =>
      request<Sanction>("POST", "/api/admin/sanctions", s),
    lift: (id: string) => request<Sanction>("POST", `/api/admin/sanctions/${encodeURIComponent(id)}/lift`),
    voidMatch: (matchId: string, reason: string) => request<void>("POST", "/api/admin/voids", { matchId, reason }),
  },
};
