import { AppLayout } from "@/components/layout";
import { Skeleton } from "@/components/ui/skeleton";

export function CaseDetailLoadingSkeleton({
  shellClass,
  panelPaddedClass,
}: {
  shellClass: string;
  panelPaddedClass: string;
}) {
  return (
    <AppLayout>
      <div className={shellClass} data-testid="case-detail-loading">
        <div className={`${panelPaddedClass} space-y-4`}>
          <div className="flex items-center gap-4">
            <Skeleton className="h-10 w-10 rounded-xl" />
            <Skeleton className="h-8 w-80 max-w-[65%]" />
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Skeleton className="h-8 w-24 rounded-lg" />
            <Skeleton className="h-8 w-20 rounded-lg" />
            <Skeleton className="h-8 w-20 rounded-lg" />
            <Skeleton className="h-8 w-24 rounded-lg" />
          </div>
        </div>

        <div className="grid gap-4 lg:grid-cols-[1.7fr,1fr]">
          <div className={`${panelPaddedClass} space-y-4`}>
            <Skeleton className="h-7 w-44 rounded-lg" />
            <div className="grid gap-3 md:grid-cols-2">
              {Array.from({ length: 4 }).map((_, index) => (
                <Skeleton key={`case-detail-overview-skeleton-${index}`} className="h-20 w-full rounded-xl" />
              ))}
            </div>
            <Skeleton className="h-24 w-full rounded-xl" />
          </div>
          <div className="space-y-4">
            <div className={`${panelPaddedClass} space-y-3`}>
              <Skeleton className="h-6 w-32 rounded-md" />
              <Skeleton className="h-10 w-full rounded-xl" />
              <Skeleton className="h-10 w-full rounded-xl" />
            </div>
            <div className={`${panelPaddedClass} space-y-3`}>
              <Skeleton className="h-6 w-36 rounded-md" />
              <Skeleton className="h-16 w-full rounded-xl" />
            </div>
          </div>
        </div>

        <div className={`${panelPaddedClass} space-y-4`}>
          <div className="flex flex-wrap gap-2">
            {Array.from({ length: 6 }).map((_, index) => (
              <Skeleton key={`case-detail-tab-skeleton-${index}`} className="h-8 w-24 rounded-lg" />
            ))}
          </div>
          <Skeleton className="h-[320px] w-full rounded-2xl" />
        </div>
      </div>
    </AppLayout>
  );
}
