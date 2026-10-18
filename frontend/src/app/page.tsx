import HeroSection from "@/components/hero-section";
import FindWorkSection from "@/components/find-work-section";
import FindPeopleSection from "@/components/find-people-section";
import WorkflowSection from "@/components/workflow-section";
import ProductShowcase from "@/components/product-showcase";
import SemanticSearchSection from "@/components/semantic-search-section";
import FinalCta from "@/components/final-cta";

export default function HomePage() {
  return (
    <main className="flex flex-col">
      <HeroSection />
      <FindWorkSection />
      <FindPeopleSection />
      <WorkflowSection />
      <ProductShowcase />
      <SemanticSearchSection />
      <FinalCta />
    </main>
  );
}
