import { FC, useState, useEffect } from "preact/compat";
import { ArrowDownIcon } from "../Icons";
import "./style.scss";
import { ReactNode } from "react";

interface AccordionProps {
  title: ReactNode
  children: ReactNode
  defaultExpanded?: boolean
  onChange?: (value: boolean) => void
}

const Accordion: FC<AccordionProps> = ({
  defaultExpanded = false,
  onChange,
  title,
  children
}) => {
  const [isOpen, setIsOpen] = useState(defaultExpanded);

  const toggleOpen = () => {
    const selection = window.getSelection();
    if (selection && selection.toString()) {
      return; // If the text is selected, cancel the execution of toggle.
    }

    setIsOpen((prev) => {
      const newState = !prev;
      onChange && onChange(newState);
      return newState;
    });
  };

  useEffect(() => {
    setIsOpen(defaultExpanded);
  }, [defaultExpanded]);

  return (
    <>
      <header
        className={`vm-accordion-header ${isOpen && "vm-accordion-header_open"}`}
        onClick={toggleOpen}
      >
        {title}
        <div className={`vm-accordion-header__arrow ${isOpen && "vm-accordion-header__arrow_open"}`}>
          <ArrowDownIcon />
        </div>
      </header>
      {isOpen && (
        <section
          className="vm-accordion-section"
          key="content"
        >
          {children}
        </section>
      )}
    </>
  );
};

export default Accordion;
