import { RootDispatcher } from "../platform/root-dispatcher";

// Root entry — the dispatcher decides what to render based on auth state:
//   no token        → /login
//   no organization → /onboarding
//   has org+branch  → /h/:org/:branch/baslangic
export default function RootPage() {
  return <RootDispatcher />;
}
