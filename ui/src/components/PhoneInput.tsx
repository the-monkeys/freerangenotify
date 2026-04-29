/**
 * PhoneInput — Country-code-aware phone number input.
 *
 * Features:
 * - Auto-detects user's country from browser timezone on mount.
 * - Searchable country dropdown with flag emojis and dial codes.
 * - Emits full E.164-compatible phone strings (dialCode + localNumber).
 * - Syncs initial value back to parent on mount (no "empty submit" bug).
 *
 * @example
 *   <PhoneInput
 *     id="user-phone"
 *     value={phone}
 *     onChange={setPhone}
 *     placeholder="9876543210"
 *   />
 */

import { useState, useEffect, useMemo, useCallback, useRef } from "react";
import { Check, ChevronsUpDown } from "lucide-react";
import { cn } from "../lib/utils";
import { Button } from "./ui/button";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "./ui/command";
import { Popover, PopoverContent, PopoverTrigger } from "./ui/popover";
import { Input } from "./ui/input";
import { COUNTRIES, getFlagEmoji } from "../lib/countries";
import {
  detectDefaultCountry,
  findCountryByCode,
  parsePhoneValue,
} from "../lib/phoneUtils";

// ─── Types ────────────────────────────────────────────────────────────────────

export interface PhoneInputProps {
  /** HTML id for the number input element. */
  id?: string;
  /** Full phone string, e.g. "+919876543210". */
  value?: string;
  /** Called with full E.164 string on every change. */
  onChange?: (value: string) => void;
  /** Placeholder for the local number input. */
  placeholder?: string;
  /** Disables both the dropdown and the text input. */
  disabled?: boolean;
}

// ─── Component ────────────────────────────────────────────────────────────────

export function PhoneInput({
  id,
  value = "",
  onChange,
  placeholder = "Phone number",
  disabled,
}: PhoneInputProps) {
  const [open, setOpen] = useState(false);
  const hasInitialized = useRef(false);

  // Compute initial state from value (if present) or timezone detection.
  const initialState = useMemo(() => {
    if (value) {
      const parsed = parsePhoneValue(value);
      if (parsed.countryCode) {
        return { country: parsed.countryCode, local: parsed.localNumber };
      }
      return { country: detectDefaultCountry(), local: value };
    }
    return { country: detectDefaultCountry(), local: "" };
  }, []); // eslint-disable-line react-hooks/exhaustive-deps — intentionally run once

  const [selectedCountry, setSelectedCountry] = useState(initialState.country);
  const [localNumber, setLocalNumber] = useState(initialState.local);

  // On mount: if value was empty, push the detected dial code back to parent.
  // This prevents the "empty submit" bug where the UI shows "+91" but the
  // form state is still "".
  useEffect(() => {
    if (hasInitialized.current) return;
    hasInitialized.current = true;

    if (!value && onChange) {
      const defaultCountry = findCountryByCode(selectedCountry);
      if (defaultCountry) {
        onChange(defaultCountry.dialCode);
      }
    }
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // Derive the current country object (with fallback).
  const country = useMemo(
    () => findCountryByCode(selectedCountry) ?? COUNTRIES[0],
    [selectedCountry],
  );

  const handleCountryChange = useCallback(
    (code: string) => {
      setSelectedCountry(code);
      setOpen(false);
      const newCountry = findCountryByCode(code);
      if (onChange && newCountry) {
        onChange(newCountry.dialCode + localNumber);
      }
    },
    [localNumber, onChange],
  );

  const handleNumberChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const cleaned = e.target.value.replace(/[^\d\s\-()]/g, "");
      setLocalNumber(cleaned);
      if (onChange) {
        onChange(country.dialCode + cleaned);
      }
    },
    [country.dialCode, onChange],
  );

  return (
    <div className="flex gap-2">
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <Button
            variant="outline"
            role="combobox"
            aria-expanded={open}
            aria-label={`Country code: ${country.name} ${country.dialCode}`}
            className="w-[140px] justify-between px-3"
            disabled={disabled}
            data-testid="phone-country-trigger"
          >
            <span className="truncate">
              {getFlagEmoji(country.code)} {country.dialCode}
            </span>
            <ChevronsUpDown className="ml-1 h-4 w-4 shrink-0 opacity-50" />
          </Button>
        </PopoverTrigger>
        <PopoverContent className="w-[300px] p-0" align="start">
          <Command>
            <CommandInput placeholder="Search country..." />
            <CommandList>
              <CommandEmpty>No country found.</CommandEmpty>
              <CommandGroup>
                {COUNTRIES.map((c) => (
                  <CommandItem
                    key={c.code}
                    value={`${c.name} ${c.dialCode} ${c.code}`}
                    onSelect={() => handleCountryChange(c.code)}
                    data-testid={`phone-country-${c.code}`}
                  >
                    <Check
                      className={cn(
                        "mr-2 h-4 w-4",
                        selectedCountry === c.code
                          ? "opacity-100"
                          : "opacity-0",
                      )}
                    />
                    <span className="mr-2">{getFlagEmoji(c.code)}</span>
                    <span className="flex-1">{c.name}</span>
                    <span className="text-muted-foreground">
                      {c.dialCode}
                    </span>
                  </CommandItem>
                ))}
              </CommandGroup>
            </CommandList>
          </Command>
        </PopoverContent>
      </Popover>
      <Input
        id={id}
        type="tel"
        placeholder={placeholder}
        value={localNumber}
        onChange={handleNumberChange}
        className="flex-1"
        disabled={disabled}
        data-testid="phone-number-input"
      />
    </div>
  );
}
