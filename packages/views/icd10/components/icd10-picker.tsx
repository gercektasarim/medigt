"use client";

// Reusable ICD-10 picker. Drop it inside any form (visit, prescription,
// admission, discharge) — it returns the selected code via onChange.

import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useHospitalStore } from "@medigt/core/hospital";
import { icd10SearchOptions } from "@medigt/core/icd10";
import type { Icd10Code } from "@medigt/core/types";
import { TextInput } from "../../common/form-fields";

export function Icd10Picker({
  onPick,
  placeholder = "ICD-10 ara...",
}: {
  onPick: (code: Icd10Code) => void;
  placeholder?: string;
}) {
  const org = useHospitalStore((s) => s.organization);
  const orgId = org?.id ?? "";
  const [q, setQ] = useState("");
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const search = useQuery(icd10SearchOptions(orgId, q, 20));

  useEffect(() => {
    const onClickOutside = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("click", onClickOutside);
    return () => document.removeEventListener("click", onClickOutside);
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
          {(search.data ?? []).map((c) => (
            <li key={c.id}>
              <button
                type="button"
                onClick={() => {
                  onPick(c);
                  setQ("");
                  setOpen(false);
                }}
                className="flex w-full items-start gap-2 px-3 py-2 text-left text-sm hover:bg-muted"
              >
                <code className="mt-0.5 rounded bg-muted px-1.5 py-0.5 text-xs">{c.code}</code>
                <span>{c.title_tr}</span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
