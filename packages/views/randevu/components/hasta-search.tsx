"use client";

// Small typeahead used inside the new-appointment drawer. Returns the
// selected patient via onPick. Backend reuses the existing /api/patients?q=.

import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useHospitalStore } from "@medigt/core/hospital";
import { hastaListOptions } from "@medigt/core/hasta";
import type { Patient } from "@medigt/core/types";
import { TextInput } from "../../common/form-fields";

export function HastaSearch({
  onPick,
  placeholder = "Hasta ara (TC, ad, telefon)...",
}: {
  onPick: (patient: Patient) => void;
  placeholder?: string;
}) {
  const org = useHospitalStore((s) => s.organization);
  const orgId = org?.id ?? "";
  const [q, setQ] = useState("");
  const [debounced, setDebounced] = useState("");
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const t = setTimeout(() => setDebounced(q.trim()), 200);
    return () => clearTimeout(t);
  }, [q]);
  const search = useQuery(hastaListOptions(orgId, debounced));

  useEffect(() => {
    const close = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("click", close);
    return () => document.removeEventListener("click", close);
  }, []);

  return (
    <div ref={ref} className="relative">
      <TextInput
        value={q}
        onChange={(e) => {
          setQ(e.target.value);
          setOpen(true);
        }}
        onFocus={() => setOpen(true)}
        placeholder={placeholder}
      />
      {open && (search.data ?? []).length > 0 && (
        <ul className="absolute z-30 mt-1 max-h-72 w-full overflow-y-auto rounded-md border border-border bg-card shadow-lg">
          {(search.data ?? []).slice(0, 12).map((p) => (
            <li key={p.id}>
              <button
                type="button"
                onClick={() => {
                  onPick(p);
                  setQ("");
                  setOpen(false);
                }}
                className="flex w-full items-start justify-between gap-3 px-3 py-2 text-left text-sm hover:bg-muted"
              >
                <div>
                  <div className="font-medium">{p.first_name} {p.last_name}</div>
                  <div className="text-xs text-muted-foreground">
                    MRN {p.mrn}
                    {p.identifier_masked && ` · ${p.identifier_masked}`}
                    {p.phone && ` · ${p.phone}`}
                  </div>
                </div>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
