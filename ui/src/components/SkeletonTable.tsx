import React from 'react';
import { Skeleton } from './ui/skeleton';

interface SkeletonTableProps {
    rows?: number;
    columns?: number;
}

const SkeletonTable: React.FC<SkeletonTableProps> = ({ rows = 5, columns = 4 }) => {
    return (
        <div className="w-full border border-border rounded-lg overflow-hidden">
            {/* Header */}
            <div className="flex gap-4 px-4 py-3 bg-muted/50 border-b border-border">
                {Array.from({ length: columns }).map((_, i) => (
                    <Skeleton key={`h-${i}`} className="h-3 w-20" />
                ))}
            </div>
            {/* Rows */}
            {Array.from({ length: rows }).map((_, rowIdx) => (
                <div
                    key={`r-${rowIdx}`}
                    className="flex gap-4 px-4 py-3 border-b border-border last:border-0"
                >
                    {Array.from({ length: columns }).map((_, colIdx) => {
                        // Vary widths for visual variety
                        const widths = ['w-2/3', 'w-1/2', 'w-3/4', 'w-1/3', 'w-2/5'];
                        const w = widths[(rowIdx + colIdx) % widths.length];
                        return <Skeleton key={`c-${colIdx}`} className={`h-4 ${w}`} />;
                    })}
                </div>
            ))}
        </div>
    );
};

export default SkeletonTable;
