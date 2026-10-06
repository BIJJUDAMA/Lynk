import HeroSection from "@/components/hero-section";
import FindWorkSection from "@/components/find-work-section";
import FindPeopleSection from "@/components/find-people-section";
import SkillsSection from "@/components/skills-section";
import WorkflowSection from "@/components/workflow-section";
import ProductShowcase from "@/components/product-showcase";
import IntelligenceSection from "@/components/intelligence-section";
import FinalCta from "@/components/final-cta";
import SiteFooter from "@/components/site-footer";

export default function HomePage() {
  return (
    <main className="flex flex-col">
      <HeroSection />
      <FindWorkSection />
      <FindPeopleSection />
      <SkillsSection />
      <WorkflowSection />
      <ProductShowcase />
      <IntelligenceSection />
      <FinalCta />
      <SiteFooter />
    </main>
  );
}
