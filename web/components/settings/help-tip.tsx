'use client';

import { Label } from '@/components/ui/label';
import { Tooltip, TooltipTrigger, TooltipContent } from '@/components/ui/tooltip';
import { CircleHelpIcon } from 'lucide-react';

export function HelpTip({ children }: { children: React.ReactNode }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          tabIndex={-1}
          aria-label="More info"
          className="inline-flex align-middle text-muted-dim hover:text-muted-fg transition-colors focus:outline-none cursor-pointer"
        >
          <CircleHelpIcon className="size-3.5" />
        </button>
      </TooltipTrigger>
      <TooltipContent className="leading-relaxed">{children}</TooltipContent>
    </Tooltip>
  );
}

export function LabelWithHelp({ label, help }: { label: string; help: React.ReactNode }) {
  return (
    <Label className="text-[0.8rem] font-semibold mb-1.5 flex items-center gap-1.5">
      {label}
      <HelpTip>{help}</HelpTip>
    </Label>
  );
}
