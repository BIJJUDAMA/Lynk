import type { Metadata } from "next";
import "./globals.css";
import { AuthProvider } from "@/components/auth/AuthProvider";
import { Navbar } from "@/components/layout/Navbar";

export const metadata: Metadata = {
  title: "Lynk | Verified Student Freelance & Campus Gigs",
  description:
    "High-trust freelance and campus gig marketplace connecting verified university students with real projects, structured contracts, and peer reviews.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className="min-h-screen bg-slate-50 text-slate-900 antialiased dark:bg-slate-950 dark:text-slate-100">
        <AuthProvider>
          <div className="flex min-h-screen flex-col">
            <Navbar />
            <div className="flex-1">{children}</div>
          </div>
        </AuthProvider>
      </body>
    </html>
  );
}
