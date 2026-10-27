import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { api, type Fighter, type Instance, type Me, type ModerationStatus } from "./api";

type AppState = {
  me: Me | null;
  instance: Instance | null;
  /** 登録済みの利用者のモデレーションの状態（管理者か・利用停止中か） */
  moderation: ModerationStatus | null;
  fighters: Fighter[];
  fighterName: (id: string) => string;
  refreshMe: () => Promise<void>;
};

const AppContext = createContext<AppState | null>(null);

export function AppProvider({ children }: { children: ReactNode }) {
  const [me, setMe] = useState<Me | null>(null);
  const [moderation, setModeration] = useState<ModerationStatus | null>(null);
  const [instance, setInstance] = useState<Instance | null>(null);
  const [fighters, setFighters] = useState<Fighter[]>([]);

  const refreshMe = useCallback(async () => {
    const m = await api.me();
    setMe(m);
    setModeration(m.loggedIn && m.registered ? await api.moderation() : null);
  }, []);

  useEffect(() => {
    void refreshMe();
    void api.instance().then(setInstance);
    void api.fighters().then(setFighters);
  }, [refreshMe]);

  const value = useMemo<AppState>(() => {
    const names = new Map(fighters.map((f) => [f.id, f.name]));
    return {
      me,
      instance,
      moderation,
      fighters,
      fighterName: (id) => (id === "random" ? "おまかせ" : (names.get(id) ?? id)),
      refreshMe,
    };
  }, [me, instance, moderation, fighters, refreshMe]);

  return <AppContext.Provider value={value}>{children}</AppContext.Provider>;
}

export function useApp(): AppState {
  const ctx = useContext(AppContext);
  if (!ctx) throw new Error("AppProvider がありません");
  return ctx;
}

/** 一定間隔で fn を実行する。fn が false を返したら止める。 */
export function usePolling(fn: () => Promise<boolean | void>, intervalMs: number, deps: unknown[]) {
  useEffect(() => {
    let stopped = false;
    let timer: ReturnType<typeof setTimeout>;
    const tick = async () => {
      let keep: boolean | void = true;
      try {
        keep = await fn();
      } catch {
        // 一時的な通信エラーは次回に再試行する
      }
      if (!stopped && keep !== false) timer = setTimeout(tick, intervalMs);
    };
    void tick();
    return () => {
      stopped = true;
      clearTimeout(timer);
    };
  }, deps);
}
