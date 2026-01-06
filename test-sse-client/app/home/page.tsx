import Notifications from "@/components/notifications";

export default function HomePage() {
    return (
        <main className="min-h-screen bg-slate-900 text-slate-200 p-10">
            <h1 className="text-4xl font-bold mb-6">FreeRangeNotify SSE Client Test</h1>
            {/* The rest of your page content */}
            <div className="w-full mx-auto flex items-center justify-center ">
                <Notifications />
            </div>
        </main>
    );
}