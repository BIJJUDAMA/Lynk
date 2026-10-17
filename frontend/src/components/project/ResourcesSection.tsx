import React from "react";
import { Link2, ExternalLink, Globe, BookOpen, FolderGit2 } from "lucide-react";
import type { Project, ProjectResource, ProjectSection } from "@/types/project";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { isSafeResourceUrl, getResourceDomain, getResourceCategory } from "@/lib/project-sections";

export interface ResourcesSectionProps {
  section: ProjectSection;
  project: Project;
  resources?: ProjectResource[];
  className?: string;
}

function ResourceIcon({ category }: { category: "github" | "demo" | "docs" | "link" }) {
  switch (category) {
    case "github":
      return <FolderGit2 className="h-4 w-4" />;
    case "demo":
      return <Globe className="h-4 w-4" />;
    case "docs":
      return <BookOpen className="h-4 w-4" />;
    case "link":
    default:
      return <ExternalLink className="h-4 w-4" />;
  }
}

export function ResourcesSection({ section, resources = [], className }: ResourcesSectionProps) {
  // Filter strictly safe URLs
  const safeResources = resources.filter((res) => isSafeResourceUrl(res.url));

  return (
    <section
      id="resources-section"
      className={cn("rounded-xl border border-border bg-card p-6 shadow-none", className)}
    >
      {/* Section Header */}
      <div className="flex items-center justify-between border-b border-border pb-4">
        <div className="flex items-center gap-2">
          <Link2 className="h-4 w-4 text-muted-foreground" />
          <h2 className="text-base font-semibold text-foreground">
            {section.title || "Resources & Links"}
          </h2>
        </div>
        <Badge variant="outline" className="font-mono text-xs">
          {safeResources.length} {safeResources.length === 1 ? "link" : "links"}
        </Badge>
      </div>

      {/* Resources Grid */}
      {safeResources.length === 0 ? (
        <div className="py-8 text-center">
          <Link2 className="mx-auto h-8 w-8 text-muted-foreground/60" />
          <p className="mt-2 text-sm text-muted-foreground">No external resources linked yet.</p>
        </div>
      ) : (
        <div className="mt-6 grid grid-cols-1 gap-3 sm:grid-cols-2">
          {safeResources.map((res) => {
            const domain = getResourceDomain(res.url);
            const category = getResourceCategory(res.resource_type, res.url);

            return (
              <a
                key={res.id || res.url}
                href={res.url}
                target="_blank"
                rel="noopener noreferrer"
                className="group flex items-center justify-between rounded-lg border border-border/80 bg-muted/20 p-3.5 transition-all hover:border-foreground/30 hover:bg-muted/40"
              >
                <div className="flex items-center gap-3 min-w-0 pr-2">
                  <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md border border-border bg-surface text-muted-foreground group-hover:text-foreground">
                    <ResourceIcon category={category} />
                  </div>
                  <div className="min-w-0">
                    <p className="text-sm font-medium text-foreground truncate group-hover:text-foreground">
                      {res.label}
                    </p>
                    {domain && (
                      <p className="font-mono text-xs text-muted-foreground truncate">{domain}</p>
                    )}
                  </div>
                </div>

                <div className="flex items-center gap-2 shrink-0">
                  {res.resource_type && (
                    <span className="hidden sm:inline-flex rounded border border-border bg-card px-1.5 py-0.5 font-mono text-[10px] uppercase text-muted-foreground">
                      {res.resource_type}
                    </span>
                  )}
                  <ExternalLink className="h-3.5 w-3.5 text-muted-foreground transition-transform group-hover:translate-x-0.5 group-hover:-translate-y-0.5 group-hover:text-foreground" />
                </div>
              </a>
            );
          })}
        </div>
      )}
    </section>
  );
}
