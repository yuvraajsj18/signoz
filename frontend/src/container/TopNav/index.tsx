import { useMemo } from 'react';
import { matchPath, useHistory } from 'react-router-dom';
import HeaderRightSection from 'components/HeaderRightSection/HeaderRightSection';
import ROUTES from 'constants/routes';

import NewExplorerCTA from '../NewExplorerCTA';
import DateTimeSelector from './DateTimeSelectionV2';
import { routesToDisable, routesToSkip } from './DateTimeSelectionV2/constants';

import './TopNav.styles.scss';

function TopNav(): JSX.Element | null {
	const { location } = useHistory();

	const isRouteToSkip = useMemo(
		() =>
			routesToSkip.some((route) =>
				matchPath(location.pathname, { path: route, exact: true }),
			),
		[location.pathname],
	);

	const isDisabled = useMemo(
		() =>
			routesToDisable.some((route) =>
				matchPath(location.pathname, { path: route, exact: true }),
			),
		[location.pathname],
	);

	const isSignUpPage = useMemo(
		() => matchPath(location.pathname, { path: ROUTES.SIGN_UP, exact: true }),
		[location.pathname],
	);

	const isHomePage = useMemo(
		() => matchPath(location.pathname, { path: ROUTES.HOME, exact: true }),
		[location.pathname],
	);

	const isNewAlertsLandingPage = useMemo(
		() =>
			matchPath(location.pathname, { path: ROUTES.ALERTS_NEW, exact: true }) &&
			!location.search,
		[location.pathname, location.search],
	);

	if (
		isSignUpPage ||
		isDisabled ||
		(isRouteToSkip && !isHomePage) ||
		isNewAlertsLandingPage
	) {
		return null;
	}

	if (isHomePage) {
		return (
			<div className="top-nav-container">
				<HeaderRightSection
					enableShare
					enableFeedback
					enableAnnouncements={false}
				/>
			</div>
		);
	}

	return !isRouteToSkip ? (
		<div className="top-nav-container">
			<NewExplorerCTA />
			<DateTimeSelector showAutoRefresh />
			<HeaderRightSection enableShare enableFeedback enableAnnouncements={false} />
		</div>
	) : null;
}

export default TopNav;
