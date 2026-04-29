import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { ChevronDown, Funnel, RotateCcw } from "lucide-react";
import { useEffect, useMemo, useState, type ReactNode } from "react";

type FilterSummaryItem = {
  key: string;
  label: string;
};

type CollapsibleFiltersPanelProps = {
  storageKey: string;
  title?: string;
  activeFiltersCount: number;
  summaryItems: FilterSummaryItem[];
  onReset: () => void;
  children: ReactNode;
};

function readCollapsedState(storageKey: string): boolean {
  if (typeof window === "undefined") {
    return true;
  }
  try {
    const raw = window.localStorage.getItem(storageKey);
    if (!raw) {
      return true;
    }
    return raw === "true" || raw === "1";
  } catch {
    return true;
  }
}

export function CollapsibleFiltersPanel({
  storageKey,
  title = "Filters",
  activeFiltersCount,
  summaryItems,
  onReset,
  children,
}: CollapsibleFiltersPanelProps) {
  const [collapsed, setCollapsed] = useState(() => readCollapsedState(storageKey));
  const contentId = useMemo(
    () => `filters-panel-${storageKey.replace(/[^a-zA-Z0-9-_]/g, "-")}`,
    [storageKey],
  );

  useEffect(() => {
    try {
      window.localStorage.setItem(storageKey, collapsed ? "true" : "false");
    } catch {
      // Ignore storage write errors (private mode / restricted env).
    }
  }, [collapsed, storageKey]);

  const maxSummaryItems = 6;
  const visibleSummaryItems = summaryItems.slice(0, maxSummaryItems);
  const hiddenSummaryItemsCount = Math.max(0, summaryItems.length - maxSummaryItems);

  return (
    <div>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2">
          <div className="inline-flex items-center gap-2 text-sm text-white">
            <Funnel size={15} className="text-[#66ff4c]" />
            <span className="font-medium">{title}</span>
          </div>
          <Badge className="h-6 rounded-md border border-[#2a2c3c] bg-[#0b0c10] px-2 text-[11px] font-medium text-[#9ca3af]">
            Active: {activeFiltersCount}
          </Badge>
        </div>
        <div className="flex items-center gap-2">
          {activeFiltersCount > 0 ? (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-8 gap-1.5 rounded-md border border-[#2a2c3c] bg-[#0b0c10] px-3 text-xs font-medium text-[#9ca3af] hover:border-[#4b5168] hover:bg-[#171b2a] hover:text-[#d1d5db]"
              onClick={onReset}
            >
              <RotateCcw size={13} />
              Reset
            </Button>
          ) : null}
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="h-8 w-8 rounded-md border border-[#2a2c3c] bg-[#0b0c10] text-[#d1d5db] hover:border-[#4b5168] hover:bg-[#171b2a]"
            aria-expanded={!collapsed}
            aria-controls={contentId}
            aria-label={collapsed ? "Expand filters" : "Collapse filters"}
            onClick={() => setCollapsed((prev) => !prev)}
          >
            <ChevronDown size={14} className={`transition-transform duration-200 ${collapsed ? "" : "rotate-180"}`} />
          </Button>
        </div>
      </div>

      {collapsed ? (
        <div className="mt-3 flex flex-wrap items-center gap-2">
          {visibleSummaryItems.length > 0 ? (
            <>
              {visibleSummaryItems.map((item) => (
                <Badge
                  key={item.key}
                  className="h-6 rounded-md border border-[#2a2c3c] bg-[#0b0c10] px-2 text-[11px] font-normal text-[#d1d5db]"
                >
                  {item.label}
                </Badge>
              ))}
              {hiddenSummaryItemsCount > 0 ? (
                <Badge className="h-6 rounded-md border border-[#2a2c3c] bg-[#0b0c10] px-2 text-[11px] font-normal text-[#9ca3af]">
                  +{hiddenSummaryItemsCount}
                </Badge>
              ) : null}
            </>
          ) : (
            <span className="text-xs text-[#6b7280]">No active filters</span>
          )}
        </div>
      ) : null}

      <div
        id={contentId}
        className={`grid overflow-hidden transition-[grid-template-rows,opacity,margin] duration-200 ease-out ${
          collapsed ? "mt-0 grid-rows-[0fr] opacity-0" : "mt-4 grid-rows-[1fr] opacity-100"
        }`}
      >
        <div className="min-h-0 overflow-hidden">{children}</div>
      </div>
    </div>
  );
}
