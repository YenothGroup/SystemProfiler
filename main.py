import json
import threading
from datetime import datetime
from tkinter import filedialog, messagebox
import customtkinter as ctk
from core import scan_all

ctk.set_appearance_mode("Dark")
ctk.set_default_color_theme("blue")


class SysProfilerApp(ctk.CTk):
    def __init__(self):
        super().__init__()
        self.title("System Hardware Profiler")
        self.geometry("880x640")
        self.minsize(720, 500)

        self.current_data: dict | None = None

        # Top Header Bar
        top_bar = ctk.CTkFrame(self, fg_color="transparent")
        top_bar.pack(fill="x", padx=24, pady=(20, 10))

        title_box = ctk.CTkFrame(top_bar, fg_color="transparent")
        title_box.pack(side="left")

        ctk.CTkLabel(
            title_box,
            text="Hardware Specifications",
            font=("Segoe UI", 22, "bold"),
        ).pack(anchor="w")

        self.os_subtitle = ctk.CTkLabel(
            title_box,
            text="Detecting operating system...",
            font=("Segoe UI", 13),
            text_color=("gray40", "gray70"),
        )
        self.os_subtitle.pack(anchor="w")

        # Action Buttons
        btn_box = ctk.CTkFrame(top_bar, fg_color="transparent")
        btn_box.pack(side="right", anchor="center")

        self.export_btn = ctk.CTkButton(
            btn_box,
            text="Export JSON",
            width=120,
            fg_color="transparent",
            border_width=1,
            command=self.export_json,
            state="disabled",
        )
        self.export_btn.pack(side="left", padx=(0, 10))

        self.refresh_btn = ctk.CTkButton(
            btn_box,
            text="Rescan",
            width=110,
            command=self.start_scan,
        )
        self.refresh_btn.pack(side="left")

        # Main Scrollable Content Area
        self.scroll = ctk.CTkScrollableFrame(self)
        self.scroll.pack(fill="both", expand=True, padx=24, pady=(5, 20))

        self.start_scan()

    def start_scan(self):
        self.refresh_btn.configure(state="disabled")
        self.export_btn.configure(state="disabled")
        self.os_subtitle.configure(text="Scanning hardware components...")

        for widget in self.scroll.winfo_children():
            widget.destroy()

        ctk.CTkLabel(
            self.scroll,
            text="Interrogating hardware buses...",
            font=("Segoe UI", 14),
            text_color=("gray40", "gray70"),
        ).pack(pady=60)

        threading.Thread(target=self._run_scan, daemon=True).start()

    def _run_scan(self):
        data = scan_all()
        self.after(0, lambda: self.render_results(data))

    def render_results(self, data: dict):
        self.current_data = data
        self.os_subtitle.configure(text=data.get("OS", "Unknown OS"))

        for widget in self.scroll.winfo_children():
            widget.destroy()

        # 1. Key-Value Sections (CPU, RAM, Motherboard)
        kv_sections = [
            ("Processor (CPU)", data.get("CPU", {})),
            ("Memory (RAM)", data.get("RAM", {})),
            ("Motherboard", data.get("Motherboard", {})),
        ]
        for title, item_dict in kv_sections:
            self._render_kv_card(title, item_dict)

        # 2. Multi-Item List Sections (GPU, Storage, Network, Batteries)
        list_sections = [
            ("Graphics (GPU)", data.get("GPU", []), self._format_gpu),
            ("Storage Drives", data.get("Storage", []), self._format_storage),
            ("Network Interfaces", data.get("Network", []), self._format_network),
            ("Batteries", data.get("Batteries", []), self._format_battery),
        ]
        for title, items, formatter in list_sections:
            self._render_list_card(title, items, formatter)

        self.refresh_btn.configure(state="normal")
        self.export_btn.configure(state="normal")

    def _create_section_card(self, title: str) -> ctk.CTkFrame:
        card = ctk.CTkFrame(self.scroll, corner_radius=10)
        card.pack(fill="x", pady=8, padx=4)

        ctk.CTkLabel(
            card,
            text=title,
            font=("Segoe UI", 15, "bold"),
            text_color=("#1f6aa5", "#4da3ff"),
        ).pack(anchor="w", padx=16, pady=(12, 6))

        return card

    def _render_kv_card(self, title: str, data: dict):
        card = self._create_section_card(title)
        grid = ctk.CTkFrame(card, fg_color="transparent")
        grid.pack(fill="x", padx=16, pady=(0, 12))
        grid.columnconfigure(1, weight=1)

        if not data:
            ctk.CTkLabel(grid, text="No data available", font=("Segoe UI", 13)).grid(
                row=0, column=0, sticky="w", pady=2
            )
            return

        for row_idx, (key, val) in enumerate(data.items()):
            label_text = key.replace("_", " ").title() + ":"
            ctk.CTkLabel(
                grid,
                text=label_text,
                font=("Segoe UI", 13, "bold"),
                text_color=("gray35", "gray75"),
                width=140,
                anchor="w",
            ).grid(row=row_idx, column=0, sticky="w", pady=3)

            ctk.CTkLabel(
                grid,
                text=str(val),
                font=("Segoe UI", 13),
                anchor="w",
                justify="left",
            ).grid(row=row_idx, column=1, sticky="w", padx=(10, 0), pady=3)

    def _render_list_card(self, title: str, items: list[dict], formatter):
        card = self._create_section_card(title)
        container = ctk.CTkFrame(card, fg_color="transparent")
        container.pack(fill="x", padx=16, pady=(0, 12))

        if not items:
            ctk.CTkLabel(
                container,
                text="None detected",
                font=("Segoe UI", 13),
                text_color=("gray45", "gray65"),
            ).pack(anchor="w", pady=2)
            return

        for idx, item in enumerate(items):
            sub_card = ctk.CTkFrame(container, fg_color=("gray88", "gray18"), corner_radius=6)
            sub_card.pack(fill="x", pady=(0 if idx == 0 else 6, 0))

            primary, secondary = formatter(item)

            ctk.CTkLabel(
                sub_card,
                text=primary,
                font=("Segoe UI", 13, "bold"),
                anchor="w",
            ).pack(anchor="w", padx=12, pady=(8, 2))

            ctk.CTkLabel(
                sub_card,
                text=secondary,
                font=("Segoe UI", 12),
                text_color=("gray35", "gray75"),
                anchor="w",
            ).pack(anchor="w", padx=12, pady=(0, 8))

    # --- Row Formatters ---
    @staticmethod
    def _format_gpu(g: dict) -> tuple[str, str]:
        return (
            g.get("name", "Unknown GPU"),
            f"VRAM: {g.get('vram', 'N/A')}   |   Driver / API: {g.get('driver', 'N/A')}",
        )

    @staticmethod
    def _format_storage(d: dict) -> tuple[str, str]:
        return (
            f"{d.get('model', 'Unknown Drive')} ({d.get('size', 'N/A')})",
            f"Type: {d.get('type', 'Unknown')}   |   Interface: {d.get('interface', 'Unknown')}",
        )

    @staticmethod
    def _format_network(n: dict) -> tuple[str, str]:
        status = n.get("status", "Unknown")
        return (
            f"{n.get('interface', 'NIC')} — {n.get('hardware', 'Adapter')} [{status}]",
            f"IPv4: {n.get('ipv4', 'N/A')}   |   MAC: {n.get('mac', 'N/A')}   |   Link: {n.get('speed', 'N/A')}",
        )

    @staticmethod
    def _format_battery(b: dict) -> tuple[str, str]:
        return (
            f"{b.get('name', 'Battery')} [{b.get('status', 'Unknown')}]",
            f"Full Charge: {b.get('full_capacity', 'N/A')}   |   Design: {b.get('design_capacity', 'N/A')}   |   Health: {b.get('health', 'N/A')}",
        )

    def export_json(self):
        if not self.current_data:
            return
        timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
        filepath = filedialog.asksaveasfilename(
            defaultextension=".json",
            initialfile=f"hardware_report_{timestamp}.json",
            filetypes=[("JSON Report", "*.json")],
        )
        if filepath:
            try:
                with open(filepath, "w", encoding="utf-8") as f:
                    json.dump(self.current_data, f, indent=4)
                messagebox.showinfo("Export Complete", f"Report saved to:\n{filepath}")
            except Exception as exc:
                messagebox.showerror("Export Failed", str(exc))


if __name__ == "__main__":
    app = SysProfilerApp()
    app.mainloop()