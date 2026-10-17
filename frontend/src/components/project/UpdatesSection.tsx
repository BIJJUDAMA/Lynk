import React from "react";
import { MessageSquare, Calendar, User } from "lucide-react";
import type { Project, ProjectSection } from "@/types/project";
import { cn } from "@/lib/utils";
import { formatJobDate } from "@/lib/formatters";

export interface ProjectUpdateItem {
  id?: string;
  title?: string;
  body: string;
  date?: string;
  author?: string;
}

export interface UpdatesSectionProps {
  section: ProjectSection;
  project: Project;
  className?: string;
}

export function UpdatesSection({ section, className }: UpdatesSectionProps) {
  // Extract updates from section config or content
  let updateItems: ProjectUpdateItem[] = [];

  if (Array.isArray(section.config?.updates)) {
    updateItems = (section.config.updates as Record<string, unknown>[])
      .map((u, idx) => ({
        id: String(u.id || `upd-${idx}`),
        title: typeof u.title === "string" ? u.title : undefined,
        body: typeof u.body === "string" ? u.body : typeof u.content === "string" ? u.content : "",
        date:
          typeof u.date === "string"
            ? u.date
            : typeof u.created_at === "string"
              ? u.created_at
              : undefined,
        author: typeof u.author === "string" ? u.author : undefined,
      }))
      .filter((item) => Boolean(item.body || item.title));
  } else if (section.content && section.content.trim()) {
    // If text content is provided, treat as a single or markdown announcement log
    updateItems = [
      {
        id: "main-update",
        body: section.content.trim(),
        date: section.content.length > 0 ? new Date().toISOString() : undefined,
      },
    ];
  }

  return (
    <section
      id="updates-section"
      className={cn("rounded-xl border border-border bg-card p-6 shadow-none", className)}
    >
      {/* Section Header */}
      <div className="flex items-center justify-between border-b border-border pb-4">
        <div className="flex items-center gap-2">
          <MessageSquare className="h-4 w-4 text-muted-foreground" />
          <h2 className="text-base font-semibold text-foreground">
            {section.title || "Announcements & Updates"}
          </h2>
        </div>
        {updateItems.length > 0 && (
          <span className="font-mono text-xs text-muted-foreground">
            {updateItems.length} {updateItems.length === 1 ? "entry" : "entries"}
          </span>
        )}
      </div>

      {/* Updates Timeline */}
      {updateItems.length === 0 ? (
        <div className="py-8 text-center">
          <MessageSquare className="mx-auto h-8 w-8 text-muted-foreground/60" />
          <p className="mt-2 text-sm text-muted-foreground">No announcements published yet.</p>
        </div>
      ) : (
        <div className="mt-6 space-y-4">
          {updateItems.map((item, idx) => (
            <div
              key={item.id || idx}
              className="rounded-lg border border-border/80 bg-muted/20 p-4 transition-colors"
            >
              <div className="flex flex-wrap items-center justify-between gap-2">
                {item.title ? (
                  <h3 className="text-sm font-semibold text-foreground">{item.title}</h3>
                ) : (
                  <span className="font-mono text-xs text-muted-foreground">
                    Announcement #{idx + 1}
                  </span>
                )}

                <div className="flex items-center gap-3 font-mono text-[11px] text-muted-foreground">
                  {item.author && (
                    <div className="flex items-center gap-1">
                      <User className="h-3 w-3 shrink-0" />
                      <span>{item.author}</span>
                    </div>
                  )}
                  {item.date && (
                    <div className="flex items-center gap-1">
                      <Calendar className="h-3 w-3 shrink-0" />
                      <span>{formatJobDate(item.date)}</span>
                    </div>
                  )}
                </div>
              </div>

              <div className="mt-2 text-xs text-foreground/90 leading-relaxed whitespace-pre-line">
                {item.body}
              </div>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}
