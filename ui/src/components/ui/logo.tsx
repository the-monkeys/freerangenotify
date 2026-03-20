import { cn } from "@/lib/utils";

const LogoWithName: React.FC = () => {
    return (
        <div className="flex items-center gap-2">
            <img src="/src/public/logo/logo.svg" alt="FreeRangeNotify Logo" className="w-6 h-6" />
            <p className="text-base sm:text-lg font-semibold tracking-tight">FreeRange <span className="font-extralight">Notify</span></p>
        </div>
    );
};

export const Logo: React.FC<{ className?: string }> = ({ className }) => {
    return (
        <img src="/src/public/logo/logo.svg" alt="FreeRangeNotify Logo" className={cn("w-6 h-6", className)} />
    );
}

export default LogoWithName;