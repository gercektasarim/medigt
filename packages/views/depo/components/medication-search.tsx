"use client";

// Small typeahead for medication selection inside drawers. Returns the
// picked medication via onPick.

import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useHospitalStore } from "@medigt/core/hospital";
import { medicationListOptions } from "@medigt/core/ilac";
import type { Medication } from "@medigt/core/types";
import { TextInput } from "../../common/form-fields";

export function MedicationSearch({
  onPick,
  placeholder = "İlaç ara (ad, etken madde, ATC, barkod)...",
}: {
  onPick: (medication: Medication) => void;
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

  const search = useQuery(medicationListOptions(orgId, { q: debounced, activeOnly: true }));

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
      {open && debounced.length > 0 && (search.data ?? []).length > 0 && (
        <ul className="absolute z-30 mt-1 max-h-72 w-full overflow-y-auto rounded-md border border-border bg-card shadow-lg">
          {(search.data ?? []).slice(0, 12).map((m) => (
            <li key={m.id}>
              <button
                type="button"
                onClick={() => {
                  onPick(m);
                  setQ("");
                  setOpen(false);
                }}
                className="flex w-full items-start justify-between gap-3 px-3 py-2 text-left text-sm hover:bg-muted"
              >
                <div>
                  <div className="font-medium">{m.name}</div>
                  <div className="text-xs text-muted-foreground">
                    {m.generic_name ?? ""}
                    {m.generic_name && m.strength && " · "}
                    {m.strength ?? ""}
                    {m.atc_code && ` · ${m.atc_code}`}
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
