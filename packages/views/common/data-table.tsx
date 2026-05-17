"use client";

import type { ReactNode } from "react";

export type Column<T> = {
  key: string;
  header: ReactNode;
  cell: (row: T) => ReactNode;
  className?: string;
};

export function DataTable<T>({
  rows,
  columns,
  emptyLabel = "Kayıt bulunamadı",
  rowKey,
  onRowClick,
}: {
  rows: T[];
  columns: Column<T>[];
  emptyLabel?: string;
  rowKey: (row: T) => string;
  onRowClick?: (row: T) => void;
}) {
  if (rows.length === 0) {
    return <div className="empty-state">{emptyLabel}</div>;
  }
  return (
    <div className="data-grid overflow-hidden">
      <table className="w-full text-sm">
        <thead className="border-b border-border bg-muted/40">
          <tr>
            {columns.map((c) => (
              <th key={c.key} className={"px-3 py-2 text-left font-medium text-muted-foreground " + (c.className ?? "")}>
                {c.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr
              key={rowKey(r)}
              className={"border-b border-border last:border-0 " + (onRowClick ? "cursor-pointer hover:bg-muted/30" : "")}
              onClick={onRowClick ? () => onRowClick(r) : undefined}
            >
              {columns.map((c) => (
                <td key={c.key} className={"px-3 py-2 " + (c.className ?? "")}>
                  {c.cell(r)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
