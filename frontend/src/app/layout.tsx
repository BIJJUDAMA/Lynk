import type { Metadata } from "next";
import "./globals.css";
import { AuthProvider } from "@/components/auth/AuthProvider";
import { Navbar } from "@/components/layout/Navbar";
import { SmoothScroll } from "@/components/ui/SmoothScroll";

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
      <body className="min-h-screen bg-background text-foreground font-sans antialiased selection:bg-muted selection:text-foreground">
        <SmoothScroll>
          <AuthProvider>
            <div className="flex min-h-screen flex-col">
              <Navbar />
              <div className="flex-1">{children}</div>
            </div>
          </AuthProvider>
        </SmoothScroll>
      </body>
    </html>
  );
}
