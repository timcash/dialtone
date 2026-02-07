/**
 * Stripe Payment Button Component
 *
 * Creates a beautiful centered Stripe payment link button.
 *
 * Setup Instructions:
 * 1. Create a Stripe account at https://stripe.com
 * 2. Go to Stripe Dashboard > Products > Create Product
 * 3. Add your product/service details and price
 * 4. Go to Payment Links (in sidebar) > Create payment link
 * 5. Copy the payment link URL and update STRIPE_PAYMENT_LINK below
 *
 * For test mode:
 * - Use Stripe's test mode toggle in dashboard
 * - Test card: 4242 4242 4242 4242, any future date, any CVC
 */

import type { VisualizationControl } from "../util/section";

// Replace with your actual Stripe Payment Link
// Test mode links start with: https://buy.stripe.com/test_...
// Live mode links start with: https://buy.stripe.com/...
const STRIPE_PAYMENT_LINK =
    "https://buy.stripe.com/test_5kQaEXcagaAoaC62N20kE00";

// Product configuration
const PRODUCT_CONFIG = {
    title: "Dialtone Official Robot Kit",
    description:
        "The complete hardware and software bundle for unified robotics. Includes custom high-torque servos, NATS bridge, and autonomy examples.",
    buttonText: "Get the Kit - $1,000",
    amount: "$1,000", // Display only - actual price set in Stripe dashboard
};

class StripeButton {
    private container: HTMLElement;
    private wrapper: HTMLElement | null = null;
    isVisible = true;

    constructor(container: HTMLElement) {
        this.container = container;
        this.render();
    }

    private render(): void {
        // Create wrapper with centering
        this.wrapper = document.createElement("div");
        this.wrapper.className = "stripe-wrapper";
        this.wrapper.innerHTML = `
            <div class="stripe-card">
                <div class="stripe-icon">
                    <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
                        <path d="M19 14c1.49-1.46 3-3.21 3-5.5A5.5 5.5 0 0 0 16.5 3c-1.76 0-3 .5-4.5 2-1.5-1.5-2.74-2-4.5-2A5.5 5.5 0 0 0 2 8.5c0 2.3 1.5 4.05 3 5.5l7 7Z"/>
                    </svg>
                </div>
                <h2 class="stripe-title">${PRODUCT_CONFIG.title}</h2>
                <p class="stripe-description">${PRODUCT_CONFIG.description}</p>
                <a href="${STRIPE_PAYMENT_LINK}" 
                   target="_blank" 
                   rel="noopener noreferrer" 
                   class="stripe-button">
                    <span class="stripe-button-icon">
                        <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                            <rect width="20" height="14" x="2" y="5" rx="2"/>
                            <line x1="2" x2="22" y1="10" y2="10"/>
                        </svg>
                    </span>
                    ${PRODUCT_CONFIG.buttonText}
                </a>
                <p class="stripe-powered">
                    <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                        <rect width="18" height="11" x="3" y="11" rx="2" ry="2"/>
                        <path d="M7 11V7a5 5 0 0 1 10 0v4"/>
                    </svg>
                    Secure payment by Stripe
                </p>
            </div>
        `;

        // Add styles
        this.injectStyles();

        this.container.appendChild(this.wrapper);
    }

    private injectStyles(): void {
        // Check if styles already injected
        if (document.getElementById("stripe-component-styles")) return;

        const styles = document.createElement("style");
        styles.id = "stripe-component-styles";
        styles.textContent = `
            .stripe-wrapper {
                display: flex;
                align-items: center;
                justify-content: center;
                width: 100%;
                height: 100%;
                padding: 2rem;
                box-sizing: border-box;
            }

            .stripe-card {
                background: rgba(0, 0, 0, 0.6);
                backdrop-filter: blur(20px);
                border: 1px solid rgba(255, 255, 255, 0.1);
                border-radius: 24px;
                padding: 3rem 2.5rem;
                text-align: center;
                max-width: 400px;
                width: 100%;
                box-shadow: 
                    0 4px 24px rgba(0, 0, 0, 0.4),
                    0 0 0 1px rgba(255, 255, 255, 0.05) inset;
                animation: stripe-card-appear 0.6s ease-out;
            }

            @keyframes stripe-card-appear {
                from {
                    opacity: 0;
                    transform: translateY(20px) scale(0.95);
                }
                to {
                    opacity: 1;
                    transform: translateY(0) scale(1);
                }
            }

            .stripe-icon {
                color: #635bff;
                margin-bottom: 1.5rem;
                animation: stripe-pulse 2s ease-in-out infinite;
            }

            @keyframes stripe-pulse {
                0%, 100% { transform: scale(1); opacity: 1; }
                50% { transform: scale(1.05); opacity: 0.8; }
            }

            .stripe-title {
                font-size: 1.75rem;
                font-weight: 600;
                color: #ffffff;
                margin: 0 0 0.75rem 0;
                letter-spacing: -0.02em;
            }

            .stripe-description {
                font-size: 1rem;
                color: rgba(255, 255, 255, 0.6);
                margin: 0 0 2rem 0;
                line-height: 1.5;
            }

            .stripe-button {
                display: inline-flex;
                align-items: center;
                justify-content: center;
                gap: 0.75rem;
                background: linear-gradient(135deg, #635bff 0%, #7c3aed 100%);
                color: white;
                font-size: 1.125rem;
                font-weight: 600;
                padding: 1rem 2.5rem;
                border-radius: 12px;
                text-decoration: none;
                transition: all 0.2s ease;
                box-shadow: 
                    0 4px 14px rgba(99, 91, 255, 0.4),
                    0 0 0 0 rgba(99, 91, 255, 0.4);
                position: relative;
                overflow: hidden;
            }

            .stripe-button::before {
                content: '';
                position: absolute;
                top: 0;
                left: -100%;
                width: 100%;
                height: 100%;
                background: linear-gradient(
                    90deg,
                    transparent,
                    rgba(255, 255, 255, 0.2),
                    transparent
                );
                transition: left 0.5s ease;
            }

            .stripe-button:hover {
                transform: translateY(-2px);
                box-shadow: 
                    0 8px 24px rgba(99, 91, 255, 0.5),
                    0 0 0 4px rgba(99, 91, 255, 0.2);
            }

            .stripe-button:hover::before {
                left: 100%;
            }

            .stripe-button:active {
                transform: translateY(0);
            }

            .stripe-button-icon {
                display: flex;
                align-items: center;
            }

            .stripe-powered {
                display: flex;
                align-items: center;
                justify-content: center;
                gap: 0.5rem;
                font-size: 0.8rem;
                color: rgba(255, 255, 255, 0.4);
                margin: 1.5rem 0 0 0;
            }

            /* Responsive */
            @media (max-width: 480px) {
                .stripe-card {
                    padding: 2rem 1.5rem;
                }
                .stripe-title {
                    font-size: 1.5rem;
                }
                .stripe-button {
                    padding: 0.875rem 2rem;
                    font-size: 1rem;
                }
            }
        `;
        document.head.appendChild(styles);
    }

    setVisible(visible: boolean): void {
        this.isVisible = visible;
        if (this.wrapper) {
            this.wrapper.style.opacity = visible ? "1" : "0.3";
        }
    }

    dispose(): void {
        if (this.wrapper && this.container.contains(this.wrapper)) {
            this.container.removeChild(this.wrapper);
        }
    }
}

/**
 * Mount function for SectionManager integration
 */
export function mountStripe(container: HTMLElement): VisualizationControl {
    const stripe = new StripeButton(container);
    return {
        dispose: () => stripe.dispose(),
        setVisible: (v) => stripe.setVisible(v),
    };
}
