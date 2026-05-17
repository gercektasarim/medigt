"use client";

import { useEffect, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { meOptions, sendLoginCode, useAuthStore, verifyLoginCode } from "@medigt/core/auth";
import { useHospitalStore } from "@medigt/core/hospital";
import { useNavigation } from "@medigt/core/navigation";
import { paths } from "@medigt/core/paths";
import { useT } from "@medigt/core/i18n";

type Step = "email" | "code";

export function LoginPage() {
  const t = useT();
  const qc = useQueryClient();
  const setSession = useAuthStore((s) => s.setSession);
  const setAvailable = useHospitalStore((s) => s.setAvailable);
  const setOrganization = useHospitalStore((s) => s.setOrganization);
  const setBranch = useHospitalStore((s) => s.setBranch);
  const nav = useNavigation();

  const [step, setStep] = useState<Step>("email");
  const [email, setEmail] = useState("");
  const [code, setCode] = useState("");

  const sendCode = useMutation({
    mutationFn: () => sendLoginCode({ email: email.trim() }),
    onSuccess: () => setStep("code"),
  });

  const verifyCode = useMutation({
    mutationFn: () => verifyLoginCode({ email: email.trim(), code: code.trim() }),
    onSuccess: async (data) => {
      // Set tokens FIRST so the api client picks up the Authorization header.
      setSession(data.user, {
        accessToken: data.access_token,
        refreshToken: data.refresh_token,
      });
      // Fetch /me synchronously, then route based on tenancy state. Avoids
      // the dispatcher racing against an in-flight /me query.
      const me = await qc.fetchQuery(meOptions());
      setAvailable(me.organizations, me.branches);

      const firstOrg = me.organizations[0];
      if (!firstOrg) {
        nav.replace(paths.onboarding());
        return;
      }
      setOrganization(firstOrg);
      const firstBranch = me.branches.find((b) => b.organization_id === firstOrg.id) ?? me.branches[0];
      if (firstBranch) {
        setBranch(firstBranch);
        nav.replace(paths.hospital(firstOrg.slug).branch(firstBranch.slug).baslangic());
        return;
      }
      nav.replace(paths.onboarding());
    },
  });

  // Auto-submit when 6 digits entered for a nicer flow.
  useEffect(() => {
    if (step === "code" && code.replace(/\D/g, "").length === 6 && !verifyCode.isPending) {
      verifyCode.mutate();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [code, step]);

  return (
    <div className="grid-bg relative flex min-h-screen items-center justify-center bg-background px-4">
      <div className="surface-card-elev anim-fade-up w-full max-w-sm space-y-6 p-8">
        <div className="space-y-2 text-center">
          <div className="eyebrow">MediGt HBYS</div>
          <h1 className="heading-xl text-2xl sm:text-3xl">Hoş geldiniz</h1>
          <p className="lede text-sm">{t.auth.welcome}</p>
        </div>

        {step === "email" && (
          <form
            className="space-y-4"
            onSubmit={(e) => {
              e.preventDefault();
              sendCode.mutate();
            }}
          >
            <div className="space-y-2">
              <label htmlFor="email" className="text-sm font-medium">{t.auth.email}</label>
              <input
                id="email"
                type="email"
                autoComplete="email"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                placeholder="ornek@hastane.local"
              />
            </div>
            <button
              type="submit"
              disabled={sendCode.isPending || !email.trim()}
              className="w-full rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
            >
              {sendCode.isPending ? "Gönderiliyor..." : t.auth.sendCode}
            </button>
            {sendCode.isError && (
              <p className="text-center text-sm text-[var(--critical)]">
                Kod gönderilemedi. Lütfen tekrar deneyin.
              </p>
            )}
          </form>
        )}

        {step === "code" && (
          <form
            className="space-y-4"
            onSubmit={(e) => {
              e.preventDefault();
              verifyCode.mutate();
            }}
          >
            <div className="space-y-2">
              <label htmlFor="code" className="text-sm font-medium">{t.auth.code}</label>
              <input
                id="code"
                type="text"
                inputMode="numeric"
                autoComplete="one-time-code"
                pattern="[0-9]{6}"
                maxLength={6}
                required
                autoFocus
                value={code}
                onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))}
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-center text-xl tracking-[0.5em] focus:outline-none focus:ring-2 focus:ring-ring"
                placeholder="------"
              />
              <p className="text-xs text-muted-foreground">
                {email} adresine 6 haneli kod gönderildi.{" "}
                Mailhog: <a className="underline" href="http://localhost:8025" target="_blank" rel="noreferrer">localhost:8025</a> · Dev kod: 888888
              </p>
            </div>

            <button
              type="submit"
              disabled={verifyCode.isPending || code.length !== 6}
              className="w-full rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
            >
              {verifyCode.isPending ? "Doğrulanıyor..." : t.auth.verifyCode}
            </button>
            {verifyCode.isError && (
              <p className="text-center text-sm text-[var(--critical)]">
                Kod hatalı veya süresi geçti.
              </p>
            )}
            <button
              type="button"
              onClick={() => {
                setCode("");
                setStep("email");
              }}
              className="w-full text-center text-xs text-muted-foreground hover:underline"
            >
              Farklı bir e-posta kullan
            </button>
          </form>
        )}
      </div>
      {/* MAL v1.0 attribution — license requires this notice be visible. */}
      <div className="mt-4 flex flex-col items-center gap-1 text-[10px] text-muted-foreground/80">
        <a
          href="https://github.com/gercektasarim/medigt"
          target="_blank"
          rel="noopener noreferrer"
          className="transition hover:text-muted-foreground"
        >
          MediGt teknolojisini temel alır · © Türker Aktaş
        </a>
        <div className="flex gap-3 opacity-80">
          <a href="mailto:turker.aktas81@gmail.com" className="hover:underline">
            turker.aktas81@gmail.com
          </a>
          <a href="tel:+905302889860" className="hover:underline">
            +90 530 288 98 60
          </a>
        </div>
      </div>
    </div>
  );
}
