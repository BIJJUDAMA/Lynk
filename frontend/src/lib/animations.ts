import gsap from "gsap";

/**
 * Premium editorial motion curve: fast response with smooth deceleration.
 * Apple / Linear style deceleration without lazy float.
 */
export const EDITORIAL_EASE: [number, number, number, number] = [0.16, 1, 0.3, 1];

export const editorialFadeUp = {
  hidden: { opacity: 0.2, y: 6 },
  visible: (delay: number = 0) => ({
    opacity: 1,
    y: 0,
    transition: {
      duration: 0.4,
      delay,
      ease: EDITORIAL_EASE,
    },
  }),
};

/**
 * Checks if the user prefers reduced motion.
 */
export function prefersReducedMotion(): boolean {
  if (typeof window === "undefined") return false;
  return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

/**
 * Animate hero section elements with a refined, subtle stagger.
 */
export function animateHero(container: HTMLElement | null): () => void {
  if (!container || prefersReducedMotion()) return () => {};

  const targets = container.querySelectorAll("[data-animate-hero]");
  if (targets.length === 0) return () => {};

  const ctx = gsap.context(() => {
    gsap.fromTo(
      targets,
      {
        opacity: 0,
        y: 8,
      },
      {
        opacity: 1,
        y: 0,
        duration: 0.5,
        stagger: 0.06,
        ease: "power3.out",
        clearProps: "transform",
      }
    );
  }, container);

  return () => ctx.revert();
}

/**
 * Animate a list or grid of cards with a soft stagger.
 */
export function animateStaggerList(
  container: HTMLElement | null,
  childSelector: string = "[data-animate-item]"
): () => void {
  if (!container || prefersReducedMotion()) return () => {};

  const items = container.querySelectorAll(childSelector);
  if (items.length === 0) return () => {};

  const ctx = gsap.context(() => {
    gsap.fromTo(
      items,
      {
        opacity: 0,
        y: 6,
      },
      {
        opacity: 1,
        y: 0,
        duration: 0.4,
        stagger: 0.05,
        ease: "power2.out",
        clearProps: "transform",
      }
    );
  }, container);

  return () => ctx.revert();
}

/**
 * Modal dialog entrance animation with subtle spring-like settling.
 */
export function animateModal(dialogElement: HTMLElement | null): () => void {
  if (!dialogElement || prefersReducedMotion()) return () => {};

  const ctx = gsap.context(() => {
    gsap.fromTo(
      dialogElement,
      {
        opacity: 0,
        scale: 0.96,
        y: 8,
      },
      {
        opacity: 1,
        scale: 1,
        y: 0,
        duration: 0.35,
        ease: "back.out(1.4)",
        clearProps: "transform",
      }
    );
  }, dialogElement);

  return () => ctx.revert();
}
