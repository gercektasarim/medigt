"use client";

import { useEffect, useRef, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import {
  CheckCircle2,
  Mic,
  MicOff,
  RefreshCw,
  Send,
  Sparkles,
  Stethoscope,
  Volume2,
  VolumeX,
} from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import { bransListOptions } from "@medigt/core/brans";
import {
  parseTranscript,
  submitIntake,
  type IntakeResult,
  type NLUResult,
  type NLUStep,
} from "@medigt/core/asistan";
import { useSpeechRecognition } from "./use-speech-recognition";
import { DashboardLayout } from "../../layout";
import {
  Field,
  PrimaryButton,
  SecondaryButton,
  SelectInput,
  TextInput,
} from "../../common/form-fields";

// Video-style intake assistant. A friendly avatar asks a fixed
// sequence of questions, each backed by a real form input so staff can
// always type. Voice (TTS) and optional speech-to-text (Web Speech
// API) layer on top — they are nice-to-have, not required.
//
// At the end we POST /api/intake which creates the patient if needed
// + an appointment in 'arrived' state. The slip page shows the
// appointment number, MRN, and the assigned doctor.

type StepKey =
  | "welcome"
  | "tc"
  | "name"
  | "birthYear"
  | "phone"
  | "complaint"
  | "specialization"
  | "confirm"
  | "result";

type AnswerState = {
  tc: string;
  firstName: string;
  lastName: string;
  birthYear: string;
  phone: string;
  complaint: string;
  specializationId: string;
  specializationName: string;
};

const empty: AnswerState = {
  tc: "",
  firstName: "",
  lastName: "",
  birthYear: "",
  phone: "",
  complaint: "",
  specializationId: "",
  specializationName: "",
};

const QUESTIONS: Record<StepKey, string> = {
  welcome: "Hoş geldiniz. Hasta kabul için size birkaç soru soracağım. Hazır olduğunuzda başla diyebilirsiniz.",
  tc: "Lütfen TC kimlik numaranızı söyleyin veya yazın.",
  name: "Adınız ve soyadınız nedir?",
  birthYear: "Doğum yılınız?",
  phone: "Telefon numaranızı paylaşır mısınız?",
  complaint: "Bugün şikayetiniz nedir? Kısaca anlatabilirsiniz.",
  specialization: "Hangi bölüm için randevu istiyorsunuz?",
  confirm: "Bilgilerinizi kontrol edelim. Onaylıyor musunuz?",
  result: "Kaydınız tamamlandı. Sıranız çağrılana kadar bekleyebilirsiniz.",
};

const STEPS: StepKey[] = [
  "welcome",
  "tc",
  "name",
  "birthYear",
  "phone",
  "complaint",
  "specialization",
  "confirm",
  "result",
];

export function AsistanPage() {
  const org = useHospitalStore((s) => s.organization);
  const orgId = org?.id ?? "";
  const branses = useQuery(bransListOptions(orgId));

  const [step, setStep] = useState<StepKey>("welcome");
  const [answers, setAnswers] = useState<AnswerState>(empty);
  const [result, setResult] = useState<IntakeResult | null>(null);

  // Voice settings — toggleable so a staff-shared kiosk doesn't blast
  // every patient's questions audibly. TTS defaults on (the whole
  // point); STT defaults off (browser support is patchy).
  const [ttsOn, setTtsOn] = useState(true);
  const [sttOn, setSttOn] = useState(false);

  // The avatar's "speaking" pulse — flip when speechSynthesis fires.
  const [speaking, setSpeaking] = useState(false);

  // Speech recognition — only meaningful for slot-collecting steps.
  const sr = useSpeechRecognition();

  // ---------- NLU loop ----------
  //
  // The transcript pipeline: STT delivers a final transcript → we POST
  // to /api/intake/parse with the current step → parser returns a
  // ParseResult with confidence + echo. We fill the answer state, then:
  //   - confidence ≥ 0.85 → speak the echo + auto-advance after 1.2s
  //   - confidence 0.5–0.85 → speak the echo + wait for user to confirm
  //   - confidence < 0.5 → ignore (the user will retype or retry)

  // Last transcript we already processed — prevents re-parsing the same
  // utterance every render.
  const lastParsed = useRef<string>("");

  useEffect(() => {
    const txt = sr.transcript;
    if (!txt || txt === lastParsed.current) return;
    if (!sttOn) return;
    if (!stepUsesNLU(step)) return;
    lastParsed.current = txt;
    void parseTranscript(stepToNLUStep(step)!, txt).then(({ result }) => {
      applyNLU(step, result, setAnswers, branses.data ?? []);
      if (result.echo && ttsOn) speak(result.echo, setSpeaking);
      // Auto-advance only when we're highly confident and we're not on
      // the confirm step (confirm is its own state machine below).
      if (
        result.confidence >= 0.85 &&
        step !== "confirm" &&
        stepUsesNLU(step)
      ) {
        // Tiny delay so the user hears the echo before the next prompt fires.
        setTimeout(() => goNext(), 1200);
      }
      // On the confirm step, "evet" → submit, "hayır" → back.
      if (step === "confirm") {
        if (result.confirm_yes) {
          setTimeout(() => submit.mutate(), 400);
        } else if (result.confirm_no) {
          setTimeout(() => goBack(), 400);
        }
      }
    });
    // Reset the recognition so the next utterance triggers a fresh event.
    sr.reset();
  }, [sr.transcript, sttOn, step, ttsOn, branses.data]); // eslint-disable-line react-hooks/exhaustive-deps

  // Auto-start listening after the question finishes speaking (if STT is on
  // and we're on a slot-collecting step). We trigger on `speaking` going
  // back to false — that's the end-of-utterance signal from TTS.
  useEffect(() => {
    if (!sttOn) {
      sr.stop();
      return;
    }
    if (!sr.supported) return;
    if (speaking) return;
    if (!stepUsesNLU(step) && step !== "confirm") return;
    if (sr.listening) return;
    // A brief gap so the TTS's tail audio doesn't bleed into recognition.
    const t = setTimeout(() => sr.start(), 250);
    return () => clearTimeout(t);
  }, [sttOn, speaking, step, sr.supported, sr.listening]); // eslint-disable-line react-hooks/exhaustive-deps

  // Submit mutation.
  const submit = useMutation({
    mutationFn: () =>
      submitIntake({
        tc_kimlik_no: answers.tc,
        first_name: answers.firstName,
        last_name: answers.lastName,
        birth_year: Number(answers.birthYear) || undefined,
        phone: answers.phone || undefined,
        complaint: answers.complaint || undefined,
        specialization_id: answers.specializationId || undefined,
      }),
    onSuccess: (r) => {
      setResult(r);
      setStep("result");
    },
  });

  // Speak the current question whenever the step changes (and TTS is on).
  const lastSpokenStep = useRef<StepKey | null>(null);
  useEffect(() => {
    if (!ttsOn) {
      stopSpeaking();
      setSpeaking(false);
      return;
    }
    if (lastSpokenStep.current === step) return;
    lastSpokenStep.current = step;
    speak(QUESTIONS[step], setSpeaking);
  }, [step, ttsOn]);

  const goNext = () => {
    const i = STEPS.indexOf(step);
    if (i >= 0 && i < STEPS.length - 1) setStep(STEPS[i + 1]!);
  };
  const goBack = () => {
    const i = STEPS.indexOf(step);
    if (i > 0) setStep(STEPS[i - 1]!);
  };

  const reset = () => {
    setAnswers(empty);
    setResult(null);
    setStep("welcome");
  };

  return (
    <DashboardLayout>
      <div className="page-shell">
        <header className="flex flex-wrap items-end justify-between gap-2">
          <div>
            <div className="eyebrow">Hasta Kabul</div>
            <h1 className="mt-1 flex items-center gap-2 heading-lg">
              <Stethoscope className="h-5 w-5 text-primary" />
              Sesli Asistan
            </h1>
            <p className="mt-1 text-sm text-muted-foreground">
              Konuşun ya da yazın — asistan dinleyip kabul kaydını başlatır.
            </p>
          </div>
          <div className="flex items-center gap-2 text-xs">
            <button
              type="button"
              onClick={() => setTtsOn((v) => !v)}
              className={
                "inline-flex items-center gap-1 rounded-md border px-2 py-1 " +
                (ttsOn ? "border-primary text-primary" : "border-border text-muted-foreground")
              }
              aria-pressed={ttsOn}
              title="Sesli okuma"
            >
              {ttsOn ? <Volume2 className="h-3.5 w-3.5" /> : <VolumeX className="h-3.5 w-3.5" />}
              <span>Sesli</span>
            </button>
            <button
              type="button"
              onClick={() => setSttOn((v) => !v)}
              className={
                "inline-flex items-center gap-1 rounded-md border px-2 py-1 " +
                (sttOn ? "border-primary text-primary" : "border-border text-muted-foreground")
              }
              aria-pressed={sttOn}
              title="Sesli yanıt (yalnızca desteklenen tarayıcılarda)"
            >
              {sttOn ? <Mic className="h-3.5 w-3.5" /> : <MicOff className="h-3.5 w-3.5" />}
              <span>Mikrofon</span>
            </button>
          </div>
        </header>

        <div className="mt-4 grid grid-cols-1 gap-4 lg:grid-cols-[260px_1fr]">
          <Avatar speaking={speaking} listening={sr.listening} />
          <div className="rounded-lg border border-border bg-card p-5">
            <Bubble text={QUESTIONS[step]} />
            <ProgressDots step={step} />
            {sttOn && sr.supported && stepUsesNLU(step) && (
              <ListenChip
                listening={sr.listening}
                interim={sr.interim}
                transcript={sr.transcript}
                error={sr.error}
              />
            )}
            {sttOn && !sr.supported && (
              <p className="mt-2 text-xs text-amber-700 dark:text-amber-300">
                Tarayıcınız ses tanımayı desteklemiyor; lütfen Chrome veya Edge kullanın.
              </p>
            )}

            <div className="mt-4">
              {step === "welcome" && (
                <WelcomeStep onStart={() => setStep("tc")} />
              )}
              {step === "tc" && (
                <TCStep
                  value={answers.tc}
                  onChange={(v) => setAnswers({ ...answers, tc: v })}
                  onNext={goNext}
                  sttOn={sttOn}
                />
              )}
              {step === "name" && (
                <NameStep
                  first={answers.firstName}
                  last={answers.lastName}
                  onChange={(first, last) =>
                    setAnswers({ ...answers, firstName: first, lastName: last })
                  }
                  onNext={goNext}
                  onBack={goBack}
                  sttOn={sttOn}
                />
              )}
              {step === "birthYear" && (
                <BirthYearStep
                  value={answers.birthYear}
                  onChange={(v) => setAnswers({ ...answers, birthYear: v })}
                  onNext={goNext}
                  onBack={goBack}
                />
              )}
              {step === "phone" && (
                <PhoneStep
                  value={answers.phone}
                  onChange={(v) => setAnswers({ ...answers, phone: v })}
                  onNext={goNext}
                  onBack={goBack}
                />
              )}
              {step === "complaint" && (
                <ComplaintStep
                  value={answers.complaint}
                  onChange={(v) => setAnswers({ ...answers, complaint: v })}
                  onNext={goNext}
                  onBack={goBack}
                  sttOn={sttOn}
                />
              )}
              {step === "specialization" && (
                <SpecializationStep
                  branses={branses.data ?? []}
                  selectedId={answers.specializationId}
                  onSelect={(id, name) =>
                    setAnswers({ ...answers, specializationId: id, specializationName: name })
                  }
                  onNext={goNext}
                  onBack={goBack}
                />
              )}
              {step === "confirm" && (
                <ConfirmStep
                  answers={answers}
                  submitting={submit.isPending}
                  error={
                    submit.isError
                      ? submit.error instanceof Error
                        ? submit.error.message
                        : "Kayıt oluşturulamadı"
                      : null
                  }
                  onBack={goBack}
                  onSubmit={() => submit.mutate()}
                />
              )}
              {step === "result" && result && (
                <ResultStep result={result} onAnother={reset} />
              )}
            </div>
          </div>
        </div>
      </div>
    </DashboardLayout>
  );
}

// ---------- Avatar + Bubble ----------

function Avatar({ speaking, listening }: { speaking: boolean; listening: boolean }) {
  const status = speaking ? "Konuşuyor…" : listening ? "Sizi dinliyor…" : "Hazır";
  return (
    <div className="surface-card grid-bg flex flex-col items-center gap-3 bg-gradient-to-b from-[var(--accent-soft)] to-card p-4">
      <div
        className={
          "relative h-40 w-40 rounded-full bg-primary/15 flex items-center justify-center transition-shadow " +
          (speaking ? "anim-pulse-soft accent-ring-strong" : listening ? "accent-ring" : "")
        }
      >
        {/* Simple face: eyes + mouth. Mouth animates when speaking; ring pulses when listening. */}
        <div className="absolute top-12 left-10 h-3 w-3 rounded-full bg-foreground" />
        <div className="absolute top-12 right-10 h-3 w-3 rounded-full bg-foreground" />
        <div
          className={
            "absolute bottom-12 left-1/2 -translate-x-1/2 rounded-full bg-foreground transition-all duration-150 " +
            (speaking ? "h-3 w-8" : "h-1 w-10")
          }
        />
        {listening && !speaking && (
          <span className="absolute -bottom-1 right-2 flex h-5 w-5 items-center justify-center rounded-full bg-primary text-[10px] text-primary-foreground">
            <Mic className="h-3 w-3" />
          </span>
        )}
      </div>
      <div className="text-center text-xs">
        <div className="font-medium">Asistan</div>
        <div className="text-muted-foreground">{status}</div>
      </div>
    </div>
  );
}

function ListenChip({
  listening,
  interim,
  transcript,
  error,
}: {
  listening: boolean;
  interim: string;
  transcript: string;
  error: string | null;
}) {
  if (error) {
    return (
      <p className="mt-2 text-xs text-rose-700 dark:text-rose-300">
        Mikrofon hatası: {error}. İzni verdiyseniz tarayıcı sekmesini yeniden açmayı deneyin.
      </p>
    );
  }
  // Show interim transcript while the user is speaking — gives instant
  // feedback that the mic is working, and lets them rephrase mid-sentence.
  if (listening && (interim || !transcript)) {
    return (
      <div
        className="mt-3 flex items-center gap-2 rounded-md border px-3 py-1.5 text-xs"
        style={{
          borderColor: "color-mix(in oklch, var(--ring) 30%, transparent)",
          background: "var(--accent-soft)",
        }}
      >
        <span className="flex h-2 w-2 rounded-full bg-primary anim-pulse-soft" />
        <span className="font-mono text-muted-foreground">
          {interim || "dinliyorum…"}
        </span>
      </div>
    );
  }
  if (transcript) {
    return (
      <div className="mt-3 rounded-md border border-border bg-muted/40 px-3 py-1.5 text-xs">
        <span className="text-muted-foreground">Duyduğum: </span>
        <span className="font-mono">{transcript}</span>
      </div>
    );
  }
  return null;
}

function Bubble({ text }: { text: string }) {
  return (
    <div className="relative rounded-xl border border-border bg-muted/40 p-4 text-base leading-relaxed anim-fade-up">
      <div className="absolute -left-2 top-4 hidden h-4 w-4 rotate-45 border-b border-l border-border bg-muted/40 lg:block" />
      <p>{text}</p>
    </div>
  );
}

function ProgressDots({ step }: { step: StepKey }) {
  const idx = STEPS.indexOf(step);
  return (
    <div className="mt-3 flex items-center gap-1.5">
      {STEPS.map((_, i) => (
        <span
          key={i}
          className={
            "h-1.5 rounded-full transition-all " +
            (i < idx ? "w-4 bg-primary" : i === idx ? "w-6 bg-primary" : "w-3 bg-border")
          }
        />
      ))}
    </div>
  );
}

// ---------- Steps ----------

function WelcomeStep({ onStart }: { onStart: () => void }) {
  return (
    <div className="flex flex-col items-start gap-3">
      <p className="text-sm text-muted-foreground">
        Süre ~1 dakika. TC kimlik, ad-soyad, doğum yılı, telefon, şikayet ve branş soracağım.
      </p>
      <PrimaryButton type="button" onClick={onStart}>
        <span className="inline-flex items-center gap-1">
          <Sparkles className="h-4 w-4" /> Başla
        </span>
      </PrimaryButton>
    </div>
  );
}

function TCStep({
  value,
  onChange,
  onNext,
  sttOn,
}: {
  value: string;
  onChange: (v: string) => void;
  onNext: () => void;
  sttOn: boolean;
}) {
  const valid = /^\d{11}$/.test(value);
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        if (valid) onNext();
      }}
      className="space-y-3"
    >
      <Field id="a-tc" label="TC kimlik numarası" required>
        <TextInput
          id="a-tc"
          autoFocus
          inputMode="numeric"
          maxLength={11}
          required
          value={value}
          onChange={(e) => onChange(e.target.value.replace(/\D/g, ""))}
          placeholder="11 haneli"
        />
      </Field>
      {sttOn && <VoiceInputHint />}
      <div className="flex justify-end">
        <PrimaryButton type="submit" disabled={!valid}>
          Devam <Send className="ml-1 inline h-4 w-4" />
        </PrimaryButton>
      </div>
    </form>
  );
}

function NameStep({
  first,
  last,
  onChange,
  onNext,
  onBack,
  sttOn,
}: {
  first: string;
  last: string;
  onChange: (f: string, l: string) => void;
  onNext: () => void;
  onBack: () => void;
  sttOn: boolean;
}) {
  const valid = first.trim().length >= 2 && last.trim().length >= 2;
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        if (valid) onNext();
      }}
      className="space-y-3"
    >
      <div className="grid grid-cols-2 gap-3">
        <Field id="a-first" label="Ad" required>
          <TextInput id="a-first" autoFocus required value={first}
            onChange={(e) => onChange(e.target.value, last)} />
        </Field>
        <Field id="a-last" label="Soyad" required>
          <TextInput id="a-last" required value={last}
            onChange={(e) => onChange(first, e.target.value)} />
        </Field>
      </div>
      {sttOn && <VoiceInputHint />}
      <div className="flex justify-between">
        <SecondaryButton type="button" onClick={onBack}>Geri</SecondaryButton>
        <PrimaryButton type="submit" disabled={!valid}>Devam</PrimaryButton>
      </div>
    </form>
  );
}

function BirthYearStep({
  value,
  onChange,
  onNext,
  onBack,
}: {
  value: string;
  onChange: (v: string) => void;
  onNext: () => void;
  onBack: () => void;
}) {
  const n = Number(value);
  const valid = n > 1900 && n < new Date().getFullYear();
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        if (valid) onNext();
      }}
      className="space-y-3"
    >
      <Field id="a-year" label="Doğum yılı" required>
        <TextInput
          id="a-year"
          autoFocus
          required
          inputMode="numeric"
          maxLength={4}
          value={value}
          onChange={(e) => onChange(e.target.value.replace(/\D/g, ""))}
          placeholder="örn. 1985"
        />
      </Field>
      <div className="flex justify-between">
        <SecondaryButton type="button" onClick={onBack}>Geri</SecondaryButton>
        <PrimaryButton type="submit" disabled={!valid}>Devam</PrimaryButton>
      </div>
    </form>
  );
}

function PhoneStep({
  value,
  onChange,
  onNext,
  onBack,
}: {
  value: string;
  onChange: (v: string) => void;
  onNext: () => void;
  onBack: () => void;
}) {
  // Phone is optional — accept blank.
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        onNext();
      }}
      className="space-y-3"
    >
      <Field id="a-phone" label="Telefon" hint="Boş bırakabilirsiniz">
        <TextInput
          id="a-phone"
          autoFocus
          inputMode="tel"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder="+90 555 ..."
        />
      </Field>
      <div className="flex justify-between">
        <SecondaryButton type="button" onClick={onBack}>Geri</SecondaryButton>
        <PrimaryButton type="submit">Devam</PrimaryButton>
      </div>
    </form>
  );
}

function ComplaintStep({
  value,
  onChange,
  onNext,
  onBack,
  sttOn,
}: {
  value: string;
  onChange: (v: string) => void;
  onNext: () => void;
  onBack: () => void;
  sttOn: boolean;
}) {
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        onNext();
      }}
      className="space-y-3"
    >
      <Field id="a-complaint" label="Şikayet" hint="Kısa bir cümle yeterli">
        <TextInput
          id="a-complaint"
          autoFocus
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder="örn. baş ağrısı, göğüs ağrısı"
        />
      </Field>
      {sttOn && <VoiceInputHint />}
      <div className="flex justify-between">
        <SecondaryButton type="button" onClick={onBack}>Geri</SecondaryButton>
        <PrimaryButton type="submit">Devam</PrimaryButton>
      </div>
    </form>
  );
}

function SpecializationStep({
  branses,
  selectedId,
  onSelect,
  onNext,
  onBack,
}: {
  branses: Array<{ id: string; code: string; name: string }>;
  selectedId: string;
  onSelect: (id: string, name: string) => void;
  onNext: () => void;
  onBack: () => void;
}) {
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        if (selectedId) onNext();
      }}
      className="space-y-3"
    >
      <Field id="a-spec" label="Branş" required>
        <SelectInput
          id="a-spec"
          required
          value={selectedId}
          onChange={(e) => {
            const id = e.target.value;
            const item = branses.find((b) => b.id === id);
            onSelect(id, item?.name ?? "");
          }}
        >
          <option value="">— seçin —</option>
          {branses.map((b) => (
            <option key={b.id} value={b.id}>{b.name}</option>
          ))}
        </SelectInput>
      </Field>
      <div className="flex justify-between">
        <SecondaryButton type="button" onClick={onBack}>Geri</SecondaryButton>
        <PrimaryButton type="submit" disabled={!selectedId}>Devam</PrimaryButton>
      </div>
    </form>
  );
}

function ConfirmStep({
  answers,
  submitting,
  error,
  onBack,
  onSubmit,
}: {
  answers: AnswerState;
  submitting: boolean;
  error: string | null;
  onBack: () => void;
  onSubmit: () => void;
}) {
  return (
    <div className="space-y-3">
      <dl className="grid grid-cols-[7rem_1fr] gap-y-1 text-sm">
        <dt className="text-muted-foreground">TC</dt>
        <dd className="font-mono">{answers.tc}</dd>
        <dt className="text-muted-foreground">Ad Soyad</dt>
        <dd>{answers.firstName} {answers.lastName}</dd>
        <dt className="text-muted-foreground">Doğum yılı</dt>
        <dd>{answers.birthYear || "—"}</dd>
        <dt className="text-muted-foreground">Telefon</dt>
        <dd>{answers.phone || "—"}</dd>
        <dt className="text-muted-foreground">Şikayet</dt>
        <dd>{answers.complaint || "—"}</dd>
        <dt className="text-muted-foreground">Branş</dt>
        <dd>{answers.specializationName || "—"}</dd>
      </dl>
      {error && (
        <div className="rounded-md border border-rose-200 bg-rose-50 p-3 text-sm text-rose-900 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-200">
          {error}
        </div>
      )}
      <div className="flex justify-between">
        <SecondaryButton type="button" onClick={onBack} disabled={submitting}>Geri</SecondaryButton>
        <PrimaryButton type="button" onClick={onSubmit} disabled={submitting}>
          {submitting ? "Kaydediliyor…" : "Onaylıyorum, kaydı oluştur"}
        </PrimaryButton>
      </div>
    </div>
  );
}

function ResultStep({
  result,
  onAnother,
}: {
  result: IntakeResult;
  onAnother: () => void;
}) {
  return (
    <div className="space-y-3 anim-fade-up">
      <div className="flex items-center gap-2 text-emerald-700 dark:text-emerald-300">
        <CheckCircle2 className="h-5 w-5" />
        <span className="text-base font-semibold">Kaydınız tamamlandı</span>
      </div>
      <div className="kpi-card grid-bg">
        <div className="eyebrow-label">Randevu numarası</div>
        <div className="kpi-value text-4xl">{result.appointment_no}</div>
        <dl className="mt-3 grid grid-cols-[7rem_1fr] gap-y-1 text-sm">
          <dt className="text-muted-foreground">MRN</dt>
          <dd className="font-mono">{result.patient_mrn}</dd>
          {result.doctor_full_name && (
            <>
              <dt className="text-muted-foreground">Doktor</dt>
              <dd>{result.doctor_full_name}</dd>
            </>
          )}
          {result.specialization_name && (
            <>
              <dt className="text-muted-foreground">Branş</dt>
              <dd>{result.specialization_name}</dd>
            </>
          )}
          <dt className="text-muted-foreground">Saat</dt>
          <dd>{new Date(result.scheduled_at).toLocaleTimeString("tr-TR", { hour: "2-digit", minute: "2-digit" })}</dd>
        </dl>
      </div>
      <p className="text-sm text-muted-foreground">
        Lütfen poliklinik bekleme alanında sıranız çağrılana kadar bekleyiniz.
      </p>
      <div className="flex gap-2">
        <SecondaryButton type="button" onClick={() => window.print()}>
          Yazdır
        </SecondaryButton>
        <PrimaryButton type="button" onClick={onAnother}>
          <span className="inline-flex items-center gap-1">
            <RefreshCw className="h-4 w-4" /> Yeni kabul
          </span>
        </PrimaryButton>
      </div>
    </div>
  );
}

function VoiceInputHint() {
  return (
    <p className="text-xs text-muted-foreground">
      <Mic className="mr-1 inline h-3 w-3" />
      Konuşarak da cevap verebilirsiniz — yanıt otomatik form alanına yazılır.
    </p>
  );
}

// ---------- Web Speech helpers ----------

function speak(text: string, onSpeakingChange: (b: boolean) => void) {
  if (typeof window === "undefined") return;
  const synth = (window as unknown as { speechSynthesis?: SpeechSynthesis }).speechSynthesis;
  if (!synth) return;
  try {
    synth.cancel();
    const u = new SpeechSynthesisUtterance(text);
    u.lang = "tr-TR";
    u.rate = 1.0;
    u.pitch = 1.05;
    u.onstart = () => onSpeakingChange(true);
    u.onend = () => onSpeakingChange(false);
    u.onerror = () => onSpeakingChange(false);
    synth.speak(u);
  } catch {
    // SSR / private mode — silently skip.
  }
}

function stopSpeaking() {
  if (typeof window === "undefined") return;
  const synth = (window as unknown as { speechSynthesis?: SpeechSynthesis }).speechSynthesis;
  synth?.cancel();
}

// ---------- NLU step helpers ----------

// Which dialog steps actually wait for a transcript? "welcome" + "result"
// are inert; "confirm" is special-cased in the effect above (it reads
// yes/no, not slot data).
function stepUsesNLU(step: StepKey): boolean {
  return (
    step === "tc" ||
    step === "name" ||
    step === "birthYear" ||
    step === "phone" ||
    step === "complaint" ||
    step === "specialization" ||
    step === "confirm"
  );
}

// Map our local step enum to the API's NLU step enum. Identical strings
// for all current cases, but we keep the indirection so the UI can
// evolve without changing the API contract.
function stepToNLUStep(step: StepKey): NLUStep | null {
  switch (step) {
    case "tc":
      return "tc";
    case "name":
      return "name";
    case "birthYear":
      return "birthYear";
    case "phone":
      return "phone";
    case "complaint":
      return "complaint";
    case "specialization":
      return "specialization";
    case "confirm":
      return "confirm";
    default:
      return null;
  }
}

// applyNLU merges the parser result into the answer state. The shape
// of `result` varies by step, so this function knows which fields to
// write for each one. branses is included for the rare case where the
// fuzzy-matcher returned an ID we want to verify against the catalog.
function applyNLU(
  step: StepKey,
  result: NLUResult,
  setAnswers: (fn: (a: AnswerState) => AnswerState) => void,
  branses: Array<{ id: string; name: string }>,
) {
  if (result.confidence < 0.5) return;
  switch (step) {
    case "tc":
      if (result.tc) setAnswers((a) => ({ ...a, tc: result.tc! }));
      return;
    case "name":
      if (result.first_name || result.last_name) {
        setAnswers((a) => ({
          ...a,
          firstName: result.first_name ?? a.firstName,
          lastName: result.last_name ?? a.lastName,
        }));
      }
      return;
    case "birthYear":
      if (result.birth_year) {
        setAnswers((a) => ({ ...a, birthYear: String(result.birth_year) }));
      }
      return;
    case "phone":
      if (result.phone) setAnswers((a) => ({ ...a, phone: result.phone! }));
      return;
    case "complaint":
      if (result.complaint) setAnswers((a) => ({ ...a, complaint: result.complaint! }));
      return;
    case "specialization":
      if (result.specialization_id) {
        const match = branses.find((b) => b.id === result.specialization_id);
        setAnswers((a) => ({
          ...a,
          specializationId: result.specialization_id!,
          specializationName: match?.name ?? result.specialization_name ?? "",
        }));
      }
      return;
  }
}

