"use client";

import { useCallback, useEffect, useRef, useState } from "react";

// Web Speech API isn't in lib.dom out of the box on all TS versions.
// We declare the minimal shape we use so this file is self-contained.
type RecognitionResult = { transcript: string };
type RecognitionResultList = {
  length: number;
  [i: number]: { 0: RecognitionResult; isFinal: boolean };
};
type RecognitionEvent = { results: RecognitionResultList };
type RecognitionErrorEvent = { error: string };
type SR = {
  lang: string;
  continuous: boolean;
  interimResults: boolean;
  start: () => void;
  stop: () => void;
  abort: () => void;
  onresult: ((e: RecognitionEvent) => void) | null;
  onerror: ((e: RecognitionErrorEvent) => void) | null;
  onend: (() => void) | null;
};
type SRCtor = new () => SR;

function getSRCtor(): SRCtor | null {
  if (typeof window === "undefined") return null;
  const w = window as unknown as {
    SpeechRecognition?: SRCtor;
    webkitSpeechRecognition?: SRCtor;
  };
  return w.SpeechRecognition ?? w.webkitSpeechRecognition ?? null;
}

export type UseSpeechRecognition = {
  supported: boolean;
  listening: boolean;
  transcript: string; // final transcript (last completed utterance)
  interim: string; // live interim transcript
  start: () => void;
  stop: () => void;
  reset: () => void;
  error: string | null;
};

// useSpeechRecognition wraps the Web Speech API. We use Turkish (tr-TR)
// and a single-utterance pattern: each `start()` listens until the user
// pauses, fires `onresult` with `isFinal=true`, and stops. The caller
// reads `transcript` (which only updates with final results), pushes
// it to the backend NLU, then calls `reset()` before the next prompt.
//
// Browser support: Chrome + Edge + Safari iOS. Firefox: no.
export function useSpeechRecognition(): UseSpeechRecognition {
  const Ctor = getSRCtor();
  const supported = Ctor !== null;

  const [listening, setListening] = useState(false);
  const [transcript, setTranscript] = useState("");
  const [interim, setInterim] = useState("");
  const [error, setError] = useState<string | null>(null);

  const recogRef = useRef<SR | null>(null);

  useEffect(() => {
    if (!Ctor) return;
    const r = new Ctor();
    r.lang = "tr-TR";
    r.continuous = false;
    r.interimResults = true;
    r.onresult = (e) => {
      let finalText = "";
      let interimText = "";
      for (let i = 0; i < e.results.length; i++) {
        const res = e.results[i]!;
        const txt = res[0].transcript;
        if (res.isFinal) finalText += txt;
        else interimText += txt;
      }
      if (interimText) setInterim(interimText);
      if (finalText) {
        setTranscript(finalText.trim());
        setInterim("");
      }
    };
    r.onerror = (e) => {
      setError(e.error || "speech_error");
      setListening(false);
    };
    r.onend = () => {
      setListening(false);
    };
    recogRef.current = r;
    return () => {
      r.abort();
      recogRef.current = null;
    };
  }, [Ctor]);

  const start = useCallback(() => {
    setError(null);
    setTranscript("");
    setInterim("");
    const r = recogRef.current;
    if (!r) return;
    try {
      r.start();
      setListening(true);
    } catch {
      // Already started — ignore.
    }
  }, []);

  const stop = useCallback(() => {
    recogRef.current?.stop();
    setListening(false);
  }, []);

  const reset = useCallback(() => {
    setTranscript("");
    setInterim("");
    setError(null);
  }, []);

  return { supported, listening, transcript, interim, start, stop, reset, error };
}
