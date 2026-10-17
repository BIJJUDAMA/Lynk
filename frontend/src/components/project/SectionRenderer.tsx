import React from "react";
import { FileText, BookOpen } from "lucide-react";
import type {
  Project,
  ProjectMember,
  ProjectResource,
  ProjectSection,
  SectionType,
} from "@/types/project";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import {
  filterAndSortSections,
  isSafeResourceUrl,
  getResourceDomain,
} from "@/lib/project-sections";
import { TeamSection } from "./TeamSection";
import { ResourcesSection } from "./ResourcesSection";
import { RecruitmentSection } from "./RecruitmentSection";
import { UpdatesSection } from "./UpdatesSection";

export interface SectionRendererProps {
  sections: ProjectSection[];
  project: Project;
  members?: ProjectMember[];
  resources?: ProjectResource[];
  isLeadOrMaintainer?: boolean;
  isMember?: boolean;
  canApply?: boolean;
  isAuthenticated?: boolean;
  isVerified?: boolean;
  onApplyClick?: () => void;
  onManageTeam?: () => void;
  className?: string;
}

// ----------------------------------------------------------------------------
// Overview Section (Trusted Platform Component)
// ----------------------------------------------------------------------------
interface OverviewSectionProps {
  section: ProjectSection;
  project: Project;
  className?: string;
}

function OverviewSection({ section, project, className }: OverviewSectionProps) {
  const content = section.content?.trim() || project.description?.trim();
  const goals = Array.isArray(section.config?.goals) ? (section.config.goals as string[]) : [];

  return (
    <section
      id="overview-section"
      className={cn("rounded-xl border border-border bg-card p-6 shadow-none", className)}
    >
      <div className="flex items-center justify-between border-b border-border pb-4">
        <div className="flex items-center gap-2">
          <FileText className="h-4 w-4 text-muted-foreground" />
          <h2 className="text-base font-semibold text-foreground">
            {section.title || "Project Overview"}
          </h2>
        </div>
        {project.category && (
          <Badge variant="outline" className="font-mono text-xs">
            {project.category}
          </Badge>
        )}
      </div>

      <div className="mt-4 text-sm text-foreground/90 leading-relaxed whitespace-pre-line">
        {content || "No detailed overview provided yet."}
      </div>

      {goals.length > 0 && (
        <div className="mt-6 border-t border-border/60 pt-4">
          <h3 className="font-mono text-xs uppercase tracking-wider text-muted-foreground">
            Core Milestones &amp; Goals
          </h3>
          <ul className="mt-2.5 space-y-2">
            {goals.map((goal, idx) => (
              <li
                key={idx}
                className="flex items-start gap-2 text-xs text-foreground/90 leading-relaxed"
              >
                <span className="font-mono text-muted-foreground select-none">[ {idx + 1} ]</span>
                <span>{goal}</span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </section>
  );
}

// ----------------------------------------------------------------------------
// Docs Section (Trusted Platform Component)
// ----------------------------------------------------------------------------
interface DocsSectionProps {
  section: ProjectSection;
  project: Project;
  className?: string;
}

function DocsSection({ section, className }: DocsSectionProps) {
  const content = section.content?.trim();
  const docLinks = Array.isArray(section.config?.links)
    ? (section.config.links as { label?: string; url?: string }[])
    : [];
  const safeDocLinks = docLinks.filter(
    (l) => typeof l.url === "string" && isSafeResourceUrl(l.url)
  );

  return (
    <section
      id="docs-section"
      className={cn("rounded-xl border border-border bg-card p-6 shadow-none", className)}
    >
      <div className="flex items-center justify-between border-b border-border pb-4">
        <div className="flex items-center gap-2">
          <BookOpen className="h-4 w-4 text-muted-foreground" />
          <h2 className="text-base font-semibold text-foreground">
            {section.title || "Documentation"}
          </h2>
        </div>
      </div>

      {content && (
        <div className="mt-4 text-sm text-foreground/90 leading-relaxed whitespace-pre-line font-mono text-xs bg-muted/20 p-4 rounded-lg border border-border/80">
          {content}
        </div>
      )}

      {safeDocLinks.length > 0 && (
        <div className="mt-4 space-y-2">
          {safeDocLinks.map((link, idx) => {
            const domain = getResourceDomain(link.url);
            return (
              <a
                key={idx}
                href={link.url}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center justify-between rounded-md border border-border/80 bg-muted/20 p-2.5 text-xs text-foreground hover:bg-muted/40 transition-colors"
              >
                <span className="font-medium truncate">{link.label || link.url}</span>
                {domain && (
                  <span className="font-mono text-[11px] text-muted-foreground truncate ml-2">
                    {domain}
                  </span>
                )}
              </a>
            );
          })}
        </div>
      )}

      {!content && safeDocLinks.length === 0 && (
        <div className="py-8 text-center">
          <BookOpen className="mx-auto h-8 w-8 text-muted-foreground/60" />
          <p className="mt-2 text-sm text-muted-foreground">
            No documentation published for this workspace yet.
          </p>
        </div>
      )}
    </section>
  );
}

// ----------------------------------------------------------------------------
// Section Registry Definition
// ----------------------------------------------------------------------------
type SectionComponentProps = {
  section: ProjectSection;
  project: Project;
  members?: ProjectMember[];
  resources?: ProjectResource[];
  isLeadOrMaintainer?: boolean;
  isMember?: boolean;
  canApply?: boolean;
  isAuthenticated?: boolean;
  isVerified?: boolean;
  onApplyClick?: () => void;
  onManageTeam?: () => void;
};

const SECTION_REGISTRY: Record<SectionType, React.ComponentType<SectionComponentProps>> = {
  overview: (props) => <OverviewSection section={props.section} project={props.project} />,
  team: (props) => (
    <TeamSection
      section={props.section}
      project={props.project}
      members={props.members}
      isLeadOrMaintainer={props.isLeadOrMaintainer}
      onManageTeam={props.onManageTeam}
    />
  ),
  resources: (props) => (
    <ResourcesSection section={props.section} project={props.project} resources={props.resources} />
  ),
  recruitment: (props) => (
    <RecruitmentSection
      section={props.section}
      project={props.project}
      canApply={props.canApply}
      isMember={props.isMember}
      isLeadOrMaintainer={props.isLeadOrMaintainer}
      isAuthenticated={props.isAuthenticated}
      isVerified={props.isVerified}
      onApplyClick={props.onApplyClick}
    />
  ),
  updates: (props) => <UpdatesSection section={props.section} project={props.project} />,
  docs: (props) => <DocsSection section={props.section} project={props.project} />,
};

// ----------------------------------------------------------------------------
// Master Section Renderer
// ----------------------------------------------------------------------------
export function SectionRenderer({
  sections,
  project,
  members = [],
  resources = [],
  isLeadOrMaintainer = false,
  isMember = false,
  canApply = false,
  isAuthenticated = false,
  isVerified = false,
  onApplyClick,
  onManageTeam,
  className,
}: SectionRendererProps) {
  // Filter enabled sections and sort strictly by position ascending
  const orderedSections = filterAndSortSections(sections);

  if (orderedSections.length === 0) {
    return (
      <div className={cn("space-y-6", className)}>
        {/* Fallback to default overview when no sections configured */}
        <OverviewSection
          section={{
            id: "fallback-overview",
            project_id: project.id,
            section_type: "overview",
            title: "Overview",
            enabled: true,
            position: 0,
            config: {},
            content: project.description,
          }}
          project={project}
        />
      </div>
    );
  }

  return (
    <div className={cn("space-y-8", className)}>
      {orderedSections.map((sec) => {
        // Safe lookup in trusted component registry.
        // Fallback: If type is unrecognized, gracefully ignore (never render arbitrary scripts/HTML).
        const Component = SECTION_REGISTRY[sec.section_type];
        if (!Component) {
          return null;
        }

        return (
          <Component
            key={sec.id || `${sec.section_type}-${sec.position}`}
            section={sec}
            project={project}
            members={members}
            resources={resources}
            isLeadOrMaintainer={isLeadOrMaintainer}
            isMember={isMember}
            canApply={canApply}
            isAuthenticated={isAuthenticated}
            isVerified={isVerified}
            onApplyClick={onApplyClick}
            onManageTeam={onManageTeam}
          />
        );
      })}
    </div>
  );
}
