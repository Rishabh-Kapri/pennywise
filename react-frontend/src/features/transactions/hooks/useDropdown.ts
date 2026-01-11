import { useEffect, useState } from 'react';

export function useDropdown<T>(
  value: string,
  allItems: T[],
  filterFn: (items: T[], query: string) => T[],
) {
  const [isOpen, setIsOpen] = useState(false);
  const [filterQuery, setFilterQuery] = useState(value);
  const [filteredItems, setFilteredItems] = useState(allItems);

  // sync with value
  useEffect(() => {
    setFilterQuery(value);
  }, [value]);

  const filterValues = (value: string) => {
    setFilterQuery(value);
    const normalized = value.trim().toLowerCase();
    setFilteredItems(filterFn(allItems, normalized));
  };

  // useEffect(() => {
  //   console.log('PayeePopover mounted');
  //   const handleEscape = (event: KeyboardEvent) => {
  //     if (event.key === 'Escape') {
  //       setIsOpen(false);
  //     }
  //   };
  //   document.addEventListener('keydown', handleEscape);
  //
  //   return () => {
  //     document.removeEventListener('keydown', handleEscape);
  //   };
  // }, []);

  return {
    isOpen,
    setIsOpen,
    filterQuery,
    setFilterQuery,
    filteredItems,
    filterValues,
  };
}
