import type { ReactNode } from "react";
import { Plug } from "lucide-react";

import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import type { ObservableConnectorOption } from "@/lib/connectors";

type ObservableConnectorMenuProps = {
  options: ObservableConnectorOption[];
  onSelect: (option: ObservableConnectorOption) => void;
  emptyLabel: string;
  disabled?: boolean;
  triggerClassName?: string;
  contentClassName?: string;
  triggerTestId?: string;
  getItemTestId?: (option: ObservableConnectorOption) => string | undefined;
  align?: "start" | "center" | "end";
  icon?: ReactNode;
};

export function ObservableConnectorMenu({
  options,
  onSelect,
  emptyLabel,
  disabled = false,
  triggerClassName,
  contentClassName,
  triggerTestId,
  getItemTestId,
  align = "end",
  icon,
}: ObservableConnectorMenuProps) {
  const hasOptions = options.length > 0;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          className={triggerClassName || "h-8 w-8 text-[#60a5fa] hover:bg-[rgba(37,99,235,0.16)] hover:text-[#93c5fd]"}
          disabled={disabled || !hasOptions}
          data-testid={triggerTestId}
        >
          {icon || <Plug size={14} />}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align={align} className={contentClassName || "min-w-[260px]"}>
        {hasOptions ? (
          options.map((option) => {
            const categoryLabel = (() => {
              const raw = String(option.category || "").toLowerCase();
              if (!raw) return "";
              if (raw === "security") return "Security";
              if (raw === "notifications") return "Notifications";
              if (raw === "infra" || raw === "infrastructure") return "Infrastructure";
              if (raw === "standard") return "Standard";
              return raw.charAt(0).toUpperCase() + raw.slice(1);
            })();
            const isSecurity = String(option.category || "").toLowerCase() === "security";
            return (
              <DropdownMenuItem
                key={`${option.connectorId}-${option.methodId}`}
                className="flex flex-col items-start gap-0.5 text-xs"
                data-testid={getItemTestId?.(option)}
                onSelect={() => onSelect(option)}
              >
                <span className="text-[#e5e7eb]">{option.label}</span>
                <div className="flex flex-wrap items-center gap-1 text-[10px] text-[#9ca3af]">
                  {categoryLabel ? (
                    <span className={isSecurity ? "rounded px-1.5 py-0.5 bg-[rgba(34,197,94,0.12)] text-[10px] text-[#bbf7d0]" : "rounded px-1.5 py-0.5 bg-[rgba(55,65,81,0.6)] text-[10px] text-[#e5e7eb]"}>
                      {categoryLabel}
                    </span>
                  ) : null}
                  {option.action ? <span className="text-[10px] text-[#9ca3af]">action: {option.action}</span> : null}
                </div>
              </DropdownMenuItem>
            );
          })
        ) : (
          <DropdownMenuItem disabled className="text-[11px] text-[#9ca3af]">
            {emptyLabel}
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
