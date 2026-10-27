// リポジトリ内のドキュメントへのリンク。サイトは概要を伝え、正式な内容はリポジトリのドキュメントに置く。
export const REPO = "https://github.com/tacky0612/smash-hub";

/** リポジトリ内のファイル（main ブランチ）へのリンク */
export const repoFile = (path: string) => `${REPO}/blob/main/${path}`;

export const DOCS = {
  readme: repoFile("README.md"),
  vision: repoFile("docs/vision.md"),
  federation: repoFile("docs/architecture/federation.md"),
  forExistingSites: repoFile("docs/architecture/federation-for-existing-sites.md"),
  rating: repoFile("docs/domain/rating.md"),
  ratingTestVectors: repoFile("docs/domain/rating-test-vectors.csv"),
  matchRules: repoFile("docs/domain/match-rules.md"),
  fighters: repoFile("docs/domain/fighters.md"),
  policy: repoFile("docs/policy.md"),
  covenant: repoFile("docs/operations/server-covenant.md"),
  dataProtection: repoFile("docs/security/data-protection.md"),
  threatModel: repoFile("docs/security/threat-model.md"),
  roadmap: repoFile("docs/roadmap.md"),
  adr: repoFile("docs/adr/README.md"),
  contributing: repoFile("CONTRIBUTING.md"),
  license: repoFile("LICENSE"),
  backend: repoFile("backend/README.md"),
  eventGo: repoFile("backend/internal/modules/federation/domain/event.go"),
  canonicalGo: repoFile("backend/internal/shared/domain/canonical.go"),
};

export const DISCUSSIONS = `${REPO}/discussions`;
export const ISSUES = `${REPO}/issues`;
/** スマブラファンの参加を歓迎するお知らせ（Discussions の Announcements） */
export const WELCOME_DISCUSSION = `${REPO}/discussions/20`;
/** 機能の提案を受け付ける Discussions のカテゴリー */
export const IDEAS = `${REPO}/discussions/categories/ideas`;
