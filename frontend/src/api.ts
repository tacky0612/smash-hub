// バックエンド API のクライアント。

export type Fighter = { id: string; name: string };

export type Account = { id: string; displayName: string; mainFighter: string };

export type Me =
  | { loggedIn: false }
  | { loggedIn: true; registered: false }
  | { loggedIn: true; registered: true; account: Account };

export type Instance = {
  name: string;
  sourceCodeUrl: string;
  loginProviders: { dev: boolean; discord: boolean };
};

export type MatchmakingStatus = {
  state: "idle" | "waiting" | "matched";
  matchId?: string;
  waitingSeconds?: number;
  allowedDiff?: number;
};

export type Report = "win" | "lose" | "cancel";

export type MatchSide = {
  player: { id: string; displayName: string; rating: number };
  fighter: string;
  reported: boolean;
  report?: Report;
};

export type Match = {
  id: string;
  status: "in_progress" | "confirmed" | "cancelled" | "expired";
  mismatched: boolean;
  playedAt: string;
  expiresAt: string;
  winner?: string;
  player1: MatchSide;
  player2: MatchSide;
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
  register: (displayName: string, mainFighter: string) =>
    request<Account>("POST", "/api/register", { displayName, mainFighter }),
  updateProfile: (displayName: string, mainFighter: string) =>
    request<Account>("PUT", "/api/me/profile", { displayName, mainFighter }),
  search: (fighter: string, rateRange: number) =>
    request<MatchmakingStatus>("POST", "/api/matchmaking/search", { fighter, rateRange }),
  status: () => request<MatchmakingStatus>("GET", "/api/matchmaking/status"),
  cancelSearch: () => request<void>("DELETE", "/api/matchmaking/search"),
  match: (id: string) => request<Match>("GET", `/api/matches/${encodeURIComponent(id)}`),
  setRoom: (id: string, roomId: string) =>
    request<Match>("PUT", `/api/matches/${encodeURIComponent(id)}/room`, { roomId }),
  report: (id: string, result: Report) =>
    request<Match>("POST", `/api/matches/${encodeURIComponent(id)}/report`, { result }),
  history: (accountId: string) => request<Match[]>("GET", `/api/accounts/${encodeURIComponent(accountId)}/matches`),
  standing: (accountId: string) => request<Standing>("GET", `/api/accounts/${encodeURIComponent(accountId)}/rating`),
  ranking: () => request<Ranking>("GET", "/api/ranking"),
};
