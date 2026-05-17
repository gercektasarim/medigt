import type { Metadata } from "next";
import type { ReactNode } from "react";
import { Providers } from "../platform/providers";
import "./globals.css";

export const metadata: Metadata = {
  title: "MediGt — Hastane Bilgi Yönetim Sistemi",
  description: "Modern hastane bilgi yönetim sistemi",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="tr" data-palette="teal" suppressHydrationWarning>
      <body>
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
