import React from 'react';
import { Button } from './ui/button';

interface PaginationProps {
    currentPage: number;
    totalItems: number;
    pageSize: number;
    onPageChange: (page: number) => void;
}

export const Pagination: React.FC<PaginationProps> = ({
    currentPage,
    totalItems,
    pageSize,
    onPageChange,
}) => {
    const totalPages = Math.max(1, Math.ceil(totalItems / pageSize));

    if (totalItems <= pageSize) return null;

    return (
        <div className="flex items-center justify-between py-4">
            <span className="text-sm text-gray-500">
                {totalItems} total &middot; Page {currentPage} of {totalPages}
            </span>
            <div className="flex gap-2">
                <Button
                    variant="outline"
                    size="sm"
                    disabled={currentPage <= 1}
                    onClick={() => onPageChange(currentPage - 1)}
                >
                    Previous
                </Button>
                <Button
                    variant="outline"
                    size="sm"
                    disabled={currentPage >= totalPages}
                    onClick={() => onPageChange(currentPage + 1)}
                >
                    Next
                </Button>
            </div>
        </div>
    );
};
