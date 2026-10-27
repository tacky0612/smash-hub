import { ButtonLink } from "../components/ui";

export function NotFoundPage() {
  return (
    <div className="mx-auto max-w-2xl px-4 py-24 text-center">
      <h1 className="text-2xl font-bold text-slate-900">ページが見つかりません</h1>
      <p className="mt-3 text-slate-600">URL が変わったか、ページが移動した可能性があります。</p>
      <div className="mt-8">
        <ButtonLink to="/">トップへ戻る</ButtonLink>
      </div>
    </div>
  );
}
