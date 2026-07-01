import { Hero } from "@/components/hero";
import { Nav } from "@/components/nav";
import {
  Comparison,
  CTA,
  Features,
  Footer,
  HowItWorks,
  Install,
  Safety,
  Throughput,
} from "@/components/sections";

export default function Page() {
  return (
    <>
      <Nav />
      <main>
        <Hero />
        <Features />
        <HowItWorks />
        <Safety />
        <Comparison />
        <Install />
        <Throughput />
        <CTA />
      </main>
      <Footer />
    </>
  );
}
