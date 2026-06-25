import { Nav } from "@/components/nav";
import { Hero } from "@/components/hero";
import {
  Features,
  HowItWorks,
  Safety,
  Comparison,
  Install,
  Throughput,
  CTA,
  Footer,
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
