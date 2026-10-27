import { HashRouter, Route, Routes } from "react-router";
import { Layout } from "./components/Layout";
import { HomePage } from "./pages/HomePage";
import { HowItWorksPage } from "./pages/HowItWorksPage";
import { DevelopersPage } from "./pages/DevelopersPage";
import { AdoptionPage } from "./pages/AdoptionPage";
import { PolicyPage } from "./pages/PolicyPage";
import { NotFoundPage } from "./pages/NotFoundPage";

// GitHub Pages はサーバー側でパスを書き換えられないため、HashRouter（/#/how-it-works）で画面を切り替える
export function App() {
  return (
    <HashRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route index element={<HomePage />} />
          <Route path="how-it-works" element={<HowItWorksPage />} />
          <Route path="developers" element={<DevelopersPage />} />
          <Route path="adoption" element={<AdoptionPage />} />
          <Route path="policy" element={<PolicyPage />} />
          <Route path="*" element={<NotFoundPage />} />
        </Route>
      </Routes>
    </HashRouter>
  );
}
