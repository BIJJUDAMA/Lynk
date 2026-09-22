import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const badgeVariants = cva(
  "inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-mono uppercase tracking-wider transition-colors",
  {
    variants: {
      variant: {
        default: "border-transparent bg-pastel-green text-pastel-greenText",
        secondary: "border-transparent bg-muted text-foreground",
        info: "border-transparent bg-pastel-blue text-pastel-blueText",
        warning: "border-transparent bg-pastel-yellow text-pastel-yellowText",
        destructive: "border-transparent bg-pastel-red text-pastel-redText",
        outline: "border-border text-foreground bg-transparent",
        success: "border-transparent bg-pastel-green text-pastel-greenText",
        accent: "border-transparent bg-pastel-blue text-pastel-blueText",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
);

export interface BadgeProps
  extends React.HTMLAttributes<HTMLDivElement>, VariantProps<typeof badgeVariants> {}

function Badge({ className, variant, ...props }: BadgeProps) {
  return <div className={cn(badgeVariants({ variant }), className)} {...props} />;
}

export { Badge, badgeVariants };
